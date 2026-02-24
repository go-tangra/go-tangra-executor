<script lang="ts" setup>
import type { VxeGridProps } from 'shell/adapter/vxe-table';

import { h, computed } from 'vue';

import { Page, useVbenDrawer, type VbenFormProps } from 'shell/vben/common-ui';
import { LucideEye } from 'shell/vben/icons';

import { Space, Button, Tag } from 'ant-design-vue';

import { useVbenVxeGrid } from 'shell/adapter/vxe-table';
import { $t } from 'shell/locales';
import { useExecutorExecutionStore } from '../../stores/executor-execution.state';
import type { ExecutionLog } from '../../api/services';

import ExecutionDrawer from './execution-drawer.vue';

const executionStore = useExecutorExecutionStore();

const statusOptions = computed(() => [
  {
    value: 'EXECUTION_STATUS_PENDING',
    label: $t('executor.page.execution.statusPending'),
  },
  {
    value: 'EXECUTION_STATUS_RUNNING',
    label: $t('executor.page.execution.statusRunning'),
  },
  {
    value: 'EXECUTION_STATUS_COMPLETED',
    label: $t('executor.page.execution.statusCompleted'),
  },
  {
    value: 'EXECUTION_STATUS_FAILED',
    label: $t('executor.page.execution.statusFailed'),
  },
  {
    value: 'EXECUTION_STATUS_REJECTED_HASH_MISMATCH',
    label: $t('executor.page.execution.statusRejectedHash'),
  },
  {
    value: 'EXECUTION_STATUS_REJECTED_NOT_APPROVED',
    label: $t('executor.page.execution.statusRejectedNotApproved'),
  },
  {
    value: 'EXECUTION_STATUS_CLIENT_OFFLINE',
    label: $t('executor.page.execution.statusClientOffline'),
  },
]);

function statusToColor(status: string | undefined) {
  switch (status) {
    case 'EXECUTION_STATUS_COMPLETED':
      return '#52C41A';
    case 'EXECUTION_STATUS_FAILED':
      return '#FF4D4F';
    case 'EXECUTION_STATUS_RUNNING':
      return '#1890FF';
    case 'EXECUTION_STATUS_PENDING':
      return '#FAAD14';
    case 'EXECUTION_STATUS_REJECTED_HASH_MISMATCH':
    case 'EXECUTION_STATUS_REJECTED_NOT_APPROVED':
      return '#722ED1';
    case 'EXECUTION_STATUS_CLIENT_OFFLINE':
      return '#8C8C8C';
    default:
      return '#C9CDD4';
  }
}

function statusToName(status: string | undefined) {
  const option = statusOptions.value.find((o) => o.value === status);
  return option?.label ?? status ?? '';
}

function triggerTypeToName(type: string | undefined) {
  switch (type) {
    case 'TRIGGER_TYPE_CLIENT_PULL':
      return $t('executor.page.execution.triggerClientPull');
    case 'TRIGGER_TYPE_UI_PUSH':
      return $t('executor.page.execution.triggerUiPush');
    default:
      return type ?? '';
  }
}

const formOptions: VbenFormProps = {
  collapsed: false,
  showCollapseButton: false,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'clientId',
      label: $t('executor.page.execution.clientId'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'status',
      label: $t('executor.page.execution.status'),
      componentProps: {
        options: statusOptions,
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<ExecutionLog> = {
  height: 'auto',
  stripe: false,
  toolbarConfig: {
    custom: true,
    export: true,
    import: false,
    refresh: true,
    zoom: true,
  },
  exportConfig: {},
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
        const resp = await executionStore.listExecutions(
          { page: page.currentPage, pageSize: page.pageSize },
          {
            clientId: formValues?.clientId,
            status: formValues?.status,
          },
        );
        return {
          items: resp.executions ?? [],
          total: resp.total ?? 0,
        };
      },
    },
  },

  columns: [
    { title: $t('ui.table.seq'), type: 'seq', width: 50 },
    {
      title: $t('executor.page.execution.scriptName'),
      field: 'scriptName',
      minWidth: 150,
    },
    {
      title: $t('executor.page.execution.clientId'),
      field: 'clientId',
      width: 130,
    },
    {
      title: $t('executor.page.execution.triggerType'),
      field: 'triggerType',
      width: 120,
      slots: { default: 'triggerType' },
    },
    {
      title: $t('executor.page.execution.status'),
      field: 'status',
      width: 160,
      slots: { default: 'status' },
    },
    {
      title: $t('executor.page.execution.exitCode'),
      field: 'exitCode',
      width: 90,
    },
    {
      title: $t('executor.page.execution.duration'),
      field: 'durationMs',
      width: 100,
      slots: { default: 'duration' },
    },
    {
      title: $t('executor.page.execution.createdAt'),
      field: 'createTime',
      width: 160,
      sortable: true,
    },
    {
      title: $t('ui.table.action'),
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      width: 80,
    },
  ],
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions, formOptions });

const [ExecutionDrawerComponent, executionDrawerApi] = useVbenDrawer({
  connectedComponent: ExecutionDrawer,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      gridApi.query();
    }
  },
});

function handleView(row: ExecutionLog) {
  executionDrawerApi.setData({ row });
  executionDrawerApi.open();
}

function formatDuration(ms: number | undefined) {
  if (ms === undefined || ms === null) return '-';
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('executor.page.execution.title')">
      <template #triggerType="{ row }">
        <Tag>{{ triggerTypeToName(row.triggerType) }}</Tag>
      </template>
      <template #status="{ row }">
        <Tag :color="statusToColor(row.status)">
          {{ statusToName(row.status) }}
        </Tag>
      </template>
      <template #duration="{ row }">
        {{ formatDuration(row.durationMs) }}
      </template>
      <template #action="{ row }">
        <Space>
          <Button
            type="link"
            size="small"
            :icon="h(LucideEye)"
            :title="$t('executor.page.execution.view')"
            @click.stop="handleView(row)"
          />
        </Space>
      </template>
    </Grid>

    <ExecutionDrawerComponent />
  </Page>
</template>
