<script lang="ts" setup>
import type { VxeGridProps } from 'shell/adapter/vxe-table';

import { h, ref } from 'vue';

import { Page, type VbenFormProps } from 'shell/vben/common-ui';
import { LucideRefreshCw } from 'shell/vben/icons';

import {
  Space,
  Button,
  Tag,
  Badge,
  Modal,
  Input,
  notification,
} from 'ant-design-vue';

import { useVbenVxeGrid } from 'shell/adapter/vxe-table';
import { $t } from 'shell/locales';
import { useExecutorExecutionStore } from '../../stores/executor-execution.state';
import {
  MtlsCertificateService,
  ConnectedClientsService,
  type MtlsCertificate,
} from '../../api/lcm-client';

const executionStore = useExecutorExecutionStore();

// Upper bound on clients fetched for client-side pagination/filtering. Online
// status comes from a separate (in-memory) executor endpoint than the paginated
// certificate source, so the merge + status filter + paging are done here over
// the fetched set. The agent-client population is bounded well below this.
const CLIENT_FETCH_LIMIT = 1000;

function statusToColor(status: string | undefined) {
  switch (status) {
    case 'MTLS_CERTIFICATE_STATUS_ACTIVE':
      return '#52C41A';
    case 'MTLS_CERTIFICATE_STATUS_EXPIRED':
      return '#FAAD14';
    case 'MTLS_CERTIFICATE_STATUS_REVOKED':
      return '#FF4D4F';
    case 'MTLS_CERTIFICATE_STATUS_SUSPENDED':
      return '#8C8C8C';
    default:
      return '#C9CDD4';
  }
}

function statusToName(status: string | undefined) {
  switch (status) {
    case 'MTLS_CERTIFICATE_STATUS_ACTIVE':
      return $t('executor.page.client.statusActive');
    case 'MTLS_CERTIFICATE_STATUS_EXPIRED':
      return $t('executor.page.client.statusExpired');
    case 'MTLS_CERTIFICATE_STATUS_REVOKED':
      return $t('executor.page.client.statusRevoked');
    case 'MTLS_CERTIFICATE_STATUS_SUSPENDED':
      return $t('executor.page.client.statusSuspended');
    default:
      return status ?? '-';
  }
}

function certTypeToColor(certType: string | undefined) {
  switch (certType) {
    case 'MTLS_CERT_TYPE_CLIENT':
      return '#1890FF';
    case 'MTLS_CERT_TYPE_SERVER':
      return '#722ED1';
    case 'MTLS_CERT_TYPE_INTERNAL':
      return '#13C2C2';
    default:
      return '#C9CDD4';
  }
}

function certTypeToName(certType: string | undefined) {
  switch (certType) {
    case 'MTLS_CERT_TYPE_CLIENT':
      return $t('executor.page.client.certTypeClient');
    case 'MTLS_CERT_TYPE_SERVER':
      return $t('executor.page.client.certTypeServer');
    case 'MTLS_CERT_TYPE_INTERNAL':
      return $t('executor.page.client.certTypeInternal');
    default:
      return certType ?? '-';
  }
}

const formOptions: VbenFormProps = {
  collapsed: false,
  showCollapseButton: false,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'commonName',
      label: $t('executor.page.client.clientId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'connection',
      label: $t('executor.page.client.connection'),
      componentProps: {
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
        options: [
          { label: $t('executor.page.client.online'), value: 'online' },
          { label: $t('executor.page.client.offline'), value: 'offline' },
        ],
      },
    },
  ],
};

const gridOptions: VxeGridProps<MtlsCertificate> = {
  height: 'auto',
  stripe: false,
  toolbarConfig: {
    custom: true,
    export: false,
    import: false,
    refresh: true,
    zoom: true,
  },
  rowConfig: {
    isHover: true,
    keyField: 'serialNumber',
  },
  checkboxConfig: {
    highlight: true,
    range: true,
  },
  pagerConfig: {
    enabled: true,
    pageSize: 20,
    pageSizes: [10, 20, 50, 100],
  },

  proxyConfig: {
    ajax: {
      query: async ({ page }, formValues) => {
        const [certResp, connResp] = await Promise.all([
          MtlsCertificateService.list({
            commonName: formValues?.commonName,
            pageSize: CLIENT_FETCH_LIMIT,
          }),
          ConnectedClientsService.list().catch(() => ({ clients: [] })),
        ]);

        const connectedMap = new Map<string, string>();
        for (const c of connResp.clients ?? []) {
          if (c.clientId) {
            connectedMap.set(c.clientId, c.clientVersion ?? '');
          }
        }

        let items = (certResp.items ?? []).map((cert) => {
          const key = cert.commonName ?? cert.clientId ?? '';
          const version = connectedMap.get(key);
          return {
            ...cert,
            online: version !== undefined,
            clientVersion: version ?? '',
          };
        });

        // Online/offline filter (status is not part of the paginated source).
        if (formValues?.connection === 'online') {
          items = items.filter((i) => i.online);
        } else if (formValues?.connection === 'offline') {
          items = items.filter((i) => !i.online);
        }

        // Paginate the merged+filtered set ourselves so the pager works
        // regardless of which source the row/status came from.
        const total = items.length;
        const start = (page.currentPage - 1) * page.pageSize;
        return {
          items: items.slice(start, start + page.pageSize),
          total,
        };
      },
    },
  },

  columns: [
    { type: 'checkbox', width: 45, fixed: 'left' },
    { title: $t('ui.table.seq'), type: 'seq', width: 50 },
    {
      title: $t('executor.page.client.clientId'),
      field: 'commonName',
      minWidth: 200,
      sortable: true,
      slots: { default: 'commonName' },
    },
    {
      title: $t('executor.page.client.version'),
      field: 'clientVersion',
      width: 120,
      sortable: true,
      slots: { default: 'version' },
    },
    {
      title: $t('executor.page.client.certStatus'),
      field: 'status',
      width: 120,
      sortable: true,
      slots: { default: 'status' },
    },
    {
      title: $t('executor.page.client.certType'),
      field: 'certType',
      width: 120,
      sortable: true,
      slots: { default: 'certType' },
    },
    {
      title: $t('executor.page.client.issuer'),
      field: 'issuerName',
      width: 180,
      sortable: true,
    },
    {
      title: $t('ui.table.action'),
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      width: 120,
    },
  ],
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions, formOptions });

const updateTargetVersion = ref('');

function clientIdOf(row: MtlsCertificate): string {
  return row.commonName ?? row.clientId ?? '';
}

async function handleTriggerUpdate(row: MtlsCertificate) {
  const clientId = clientIdOf(row);
  if (!clientId) return;

  updateTargetVersion.value = '';

  Modal.confirm({
    title: $t('executor.page.client.updateTitle'),
    content: h('div', { style: 'margin-top: 12px' }, [
      h('div', { style: 'margin-bottom: 8px' }, [
        h('span', { class: 'font-mono text-xs' }, clientId),
      ]),
      h('div', { style: 'margin-bottom: 8px; margin-top: 12px' }, $t('executor.page.client.targetVersion')),
      h(Input, {
        placeholder: $t('executor.page.client.targetVersionPlaceholder'),
        allowClear: true,
        onChange: (e: Event) => {
          updateTargetVersion.value = (e.target as HTMLInputElement)?.value ?? '';
        },
      }),
    ]),
    async onOk() {
      try {
        const resp = await executionStore.triggerClientUpdate(
          clientId,
          updateTargetVersion.value || undefined,
        );
        if (resp.clientOnline) {
          notification.success({
            message: $t('executor.page.client.updateTriggered'),
            description: $t('executor.page.client.updateCommandSent', {
              commandId: resp.commandId,
            }),
          });
        } else {
          notification.warning({
            message: $t('executor.page.client.clientOffline'),
            description: $t('executor.page.client.clientOfflineDesc'),
          });
        }
      } catch {
        notification.error({ message: $t('executor.page.client.updateFailed') });
      }
    },
  });
}

// Force a client self-update on every selected host at once. Each host gets the
// same (optional) target version; results are tallied by online/offline/failed.
async function handleBatchUpdate() {
  const rows = (gridApi.grid?.getCheckboxRecords?.() ?? []) as MtlsCertificate[];
  const clientIds = [...new Set(rows.map(clientIdOf).filter(Boolean))];
  if (clientIds.length === 0) {
    notification.warning({ message: $t('executor.page.client.selectClients') });
    return;
  }

  updateTargetVersion.value = '';

  Modal.confirm({
    title: $t('executor.page.client.batchUpdateTitle', { count: clientIds.length }),
    content: h('div', { style: 'margin-top: 12px' }, [
      h('div', { style: 'margin-bottom: 8px; margin-top: 4px' }, $t('executor.page.client.targetVersion')),
      h(Input, {
        placeholder: $t('executor.page.client.targetVersionPlaceholder'),
        allowClear: true,
        onChange: (e: Event) => {
          updateTargetVersion.value = (e.target as HTMLInputElement)?.value ?? '';
        },
      }),
    ]),
    async onOk() {
      const version = updateTargetVersion.value || undefined;
      const results = await Promise.allSettled(
        clientIds.map((id) => executionStore.triggerClientUpdate(id, version)),
      );
      let online = 0;
      let offline = 0;
      let failed = 0;
      for (const r of results) {
        if (r.status === 'fulfilled') {
          if (r.value?.clientOnline) online += 1;
          else offline += 1;
        } else {
          failed += 1;
        }
      }
      const note = { message: $t('executor.page.client.batchUpdateResult', { online, offline, failed }) };
      if (failed > 0) notification.error(note);
      else notification.success(note);

      gridApi.grid?.clearCheckboxRow?.();
      await gridApi.query();
    },
  });
}
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('executor.page.client.title')">
      <template #toolbar-tools>
        <Button
          class="mr-2"
          type="primary"
          :icon="h(LucideRefreshCw)"
          @click="handleBatchUpdate"
        >
          {{ $t('executor.page.client.updateSelected') }}
        </Button>
      </template>
      <template #commonName="{ row }">
        <Badge :status="row.online ? 'success' : 'default'" />
        <span class="font-mono text-xs ml-1">{{ row.commonName ?? row.clientId }}</span>
      </template>
      <template #version="{ row }">
        <span v-if="row.clientVersion" class="font-mono text-xs">
          {{ row.clientVersion }}
        </span>
        <span v-else class="text-gray-400">-</span>
      </template>
      <template #status="{ row }">
        <Tag :color="statusToColor(row.status)">
          {{ statusToName(row.status) }}
        </Tag>
      </template>
      <template #certType="{ row }">
        <Tag :color="certTypeToColor(row.certType)">
          {{ certTypeToName(row.certType) }}
        </Tag>
      </template>
      <template #action="{ row }">
        <Space>
          <Button
            type="link"
            size="small"
            :icon="h(LucideRefreshCw)"
            :title="$t('executor.page.client.triggerUpdate')"
            @click.stop="handleTriggerUpdate(row)"
          >
            {{ $t('executor.page.client.update') }}
          </Button>
        </Space>
      </template>
    </Grid>
  </Page>
</template>
