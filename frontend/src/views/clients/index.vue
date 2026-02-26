<script lang="ts" setup>
import type { VxeGridProps } from 'shell/adapter/vxe-table';

import { h, ref } from 'vue';

import { Page, type VbenFormProps } from 'shell/vben/common-ui';
import { LucideRefreshCw } from 'shell/vben/icons';

import {
  Space,
  Button,
  Tag,
  Modal,
  Input,
  notification,
} from 'ant-design-vue';

import { useVbenVxeGrid } from 'shell/adapter/vxe-table';
import { $t } from 'shell/locales';
import { useExecutorExecutionStore } from '../../stores/executor-execution.state';
import {
  MtlsCertificateService,
  type MtlsCertificate,
} from '../../api/lcm-client';

const executionStore = useExecutorExecutionStore();

function statusToColor(status: string | undefined) {
  switch (status) {
    case 'ACTIVE':
      return '#52C41A';
    case 'REVOKED':
      return '#FF4D4F';
    case 'EXPIRED':
      return '#8C8C8C';
    default:
      return '#C9CDD4';
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
        const resp = await MtlsCertificateService.list({
          commonName: formValues?.commonName,
          pageSize: page.pageSize,
        });
        return {
          items: resp.items ?? [],
          total: resp.total ?? 0,
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
        <span class="font-mono text-xs">{{ row.commonName ?? row.clientId }}</span>
      </template>
      <template #status="{ row }">
        <Tag :color="statusToColor(row.status)">
          {{ row.status ?? '-' }}
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
