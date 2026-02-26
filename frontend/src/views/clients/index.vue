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
            pageSize: page.pageSize,
          }),
          ConnectedClientsService.list().catch(() => ({ clients: [] })),
        ]);

        const connectedMap = new Map<string, string>();
        for (const c of connResp.clients ?? []) {
          if (c.clientId) {
            connectedMap.set(c.clientId, c.clientVersion ?? '');
          }
        }

        const items = (certResp.items ?? []).map((cert) => {
          const key = cert.commonName ?? cert.clientId ?? '';
          const version = connectedMap.get(key);
          return {
            ...cert,
            online: version !== undefined,
            clientVersion: version ?? '',
          };
        });

        return {
          items,
          total: certResp.total ?? 0,
        };
      },
    },
  },

  columns: [
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

const [Grid] = useVbenVxeGrid({ gridOptions, formOptions });

const updateTargetVersion = ref('');

async function handleTriggerUpdate(row: MtlsCertificate) {
  const clientId = row.commonName ?? row.clientId ?? '';
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
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('executor.page.client.title')">
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
