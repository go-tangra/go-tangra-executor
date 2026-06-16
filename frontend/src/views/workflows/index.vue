<script lang="ts" setup>
import type { VxeGridProps } from 'shell/adapter/vxe-table';

import { h } from 'vue';

import { Page, useVbenDrawer, type VbenFormProps } from 'shell/vben/common-ui';
import {
  LucideEye,
  LucidePencil,
  LucideTrash2,
  LucidePlus,
  LucideCirclePlay,
} from 'shell/vben/icons';

import { notification, Space, Button, Tag } from 'ant-design-vue';

import { useVbenVxeGrid } from 'shell/adapter/vxe-table';
import { $t } from 'shell/locales';
import { useExecutorWorkflowStore } from '../../stores/executor-workflow.state';
import type { Workflow } from '../../api/services';

import WorkflowDrawer from './workflow-drawer.vue';
import RunDrawer from './run-drawer.vue';

const workflowStore = useExecutorWorkflowStore();

function enableColor(v: boolean | undefined) {
  return v ? '#52C41A' : '#8C8C8C';
}
function enableName(v: boolean | undefined) {
  return v ? 'Enabled' : 'Disabled';
}

const formOptions: VbenFormProps = {
  collapsed: false,
  showCollapseButton: false,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'name',
      label: $t('executor.page.workflow.name'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<Workflow> = {
  height: 'auto',
  stripe: false,
  toolbarConfig: { custom: true, export: true, import: false, refresh: true, zoom: true },
  exportConfig: {},
  rowConfig: { isHover: true },
  pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 50, 100] },
  proxyConfig: {
    ajax: {
      query: async ({ page }, formValues) => {
        const resp = await workflowStore.listWorkflows(
          { page: page.currentPage, pageSize: page.pageSize },
          { name: formValues?.name },
        );
        return { items: resp.workflows ?? [], total: resp.total ?? 0 };
      },
    },
  },
  columns: [
    { title: $t('ui.table.seq'), type: 'seq', width: 50 },
    { title: $t('executor.page.workflow.name'), field: 'name', minWidth: 200 },
    {
      title: $t('executor.page.workflow.description'),
      field: 'description',
      minWidth: 220,
      showOverflow: 'tooltip',
    },
    { title: $t('executor.page.workflow.version'), field: 'version', width: 90 },
    {
      title: $t('executor.page.workflow.enabled'),
      field: 'enabled',
      width: 100,
      slots: { default: 'enabled' },
    },
    { title: $t('executor.page.workflow.createdAt'), field: 'createdAt', width: 160, sortable: true },
    {
      title: $t('ui.table.action'),
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      width: 180,
    },
  ],
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions, formOptions });

const [WorkflowDrawerComponent, workflowDrawerApi] = useVbenDrawer({
  connectedComponent: WorkflowDrawer,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) gridApi.query();
  },
});

const [RunDrawerComponent, runDrawerApi] = useVbenDrawer({
  connectedComponent: RunDrawer,
});

function handleCreate() {
  workflowDrawerApi.setData({ mode: 'create' });
  workflowDrawerApi.open();
}
function handleView(row: Workflow) {
  workflowDrawerApi.setData({ row, mode: 'view' });
  workflowDrawerApi.open();
}
function handleEdit(row: Workflow) {
  workflowDrawerApi.setData({ row, mode: 'edit' });
  workflowDrawerApi.open();
}
function handleRun(row: Workflow) {
  runDrawerApi.setData({ workflow: row });
  runDrawerApi.open();
}
async function handleDelete(row: Workflow) {
  if (!row.id) return;
  try {
    await workflowStore.deleteWorkflow(row.id);
    notification.success({ message: $t('executor.page.workflow.deleteSuccess') });
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('executor.page.workflow.title')">
      <template #toolbar-tools>
        <Button class="mr-2" type="primary" :icon="h(LucidePlus)" @click="handleCreate">
          {{ $t('executor.page.workflow.create') }}
        </Button>
      </template>
      <template #enabled="{ row }">
        <Tag :color="enableColor(row.enabled)">{{ enableName(row.enabled) }}</Tag>
      </template>
      <template #action="{ row }">
        <Space>
          <Button
            type="link"
            size="small"
            :icon="h(LucideCirclePlay)"
            :title="$t('executor.page.workflow.run')"
            @click.stop="handleRun(row)"
          />
          <Button type="link" size="small" :icon="h(LucideEye)" :title="$t('ui.button.view')" @click.stop="handleView(row)" />
          <Button type="link" size="small" :icon="h(LucidePencil)" :title="$t('executor.page.workflow.edit')" @click.stop="handleEdit(row)" />
          <a-popconfirm
            :cancel-text="$t('ui.button.cancel')"
            :ok-text="$t('ui.button.ok')"
            :title="$t('executor.page.workflow.confirmDelete')"
            @confirm="handleDelete(row)"
          >
            <Button danger type="link" size="small" :icon="h(LucideTrash2)" :title="$t('executor.page.workflow.delete')" />
          </a-popconfirm>
        </Space>
      </template>
    </Grid>

    <WorkflowDrawerComponent />
    <RunDrawerComponent />
  </Page>
</template>
