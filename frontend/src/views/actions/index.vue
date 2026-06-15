<script lang="ts" setup>
import type { VxeGridProps } from 'shell/adapter/vxe-table';

import { h } from 'vue';

import { Page, useVbenDrawer, type VbenFormProps } from 'shell/vben/common-ui';
import {
  LucideEye,
  LucidePencil,
  LucideTrash2,
  LucidePlus,
} from 'shell/vben/icons';

import { notification, Space, Button, Tag } from 'ant-design-vue';

import { useVbenVxeGrid } from 'shell/adapter/vxe-table';
import { $t } from 'shell/locales';
import { useExecutorActionStore } from '../../stores/executor-action.state';
import type { Action } from '../../api/services';

import ActionDrawer from './action-drawer.vue';

const actionStore = useExecutorActionStore();

function enableBoolToColor(value: boolean | undefined) {
  return value ? '#52C41A' : '#8C8C8C';
}

function enableBoolToName(value: boolean | undefined) {
  return value ? 'Enabled' : 'Disabled';
}

function usingToColor(using: string | undefined) {
  switch (using) {
    case 'composite':
      return '#1890FF';
    case 'javascript':
      return '#D4B106';
    case 'lua':
      return '#389E0D';
    default:
      return '#8C8C8C';
  }
}

const formOptions: VbenFormProps = {
  collapsed: false,
  showCollapseButton: false,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'name',
      label: $t('executor.page.action.name'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'using',
      label: $t('executor.page.action.using'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<Action> = {
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
        const resp = await actionStore.listActions(
          { page: page.currentPage, pageSize: page.pageSize },
          {
            name: formValues?.name,
            using: formValues?.using,
          },
        );
        return {
          items: resp.actions ?? [],
          total: resp.total ?? 0,
        };
      },
    },
  },

  columns: [
    { title: $t('ui.table.seq'), type: 'seq', width: 50 },
    {
      title: $t('executor.page.action.name'),
      field: 'name',
      minWidth: 200,
    },
    {
      title: $t('executor.page.action.description'),
      field: 'description',
      minWidth: 200,
      showOverflow: 'tooltip',
    },
    {
      title: $t('executor.page.action.using'),
      field: 'using',
      width: 130,
      slots: { default: 'using' },
    },
    {
      title: $t('executor.page.action.version'),
      field: 'version',
      width: 90,
    },
    {
      title: $t('executor.page.action.enabled'),
      field: 'enabled',
      width: 100,
      slots: { default: 'enabled' },
    },
    {
      title: $t('executor.page.action.createdAt'),
      field: 'createdAt',
      width: 160,
      sortable: true,
    },
    {
      title: $t('ui.table.action'),
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      width: 140,
    },
  ],
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions, formOptions });

const [ActionDrawerComponent, actionDrawerApi] = useVbenDrawer({
  connectedComponent: ActionDrawer,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      gridApi.query();
    }
  },
});

function handleCreate() {
  actionDrawerApi.setData({ mode: 'create' });
  actionDrawerApi.open();
}

function handleView(row: Action) {
  actionDrawerApi.setData({ row, mode: 'view' });
  actionDrawerApi.open();
}

function handleEdit(row: Action) {
  actionDrawerApi.setData({ row, mode: 'edit' });
  actionDrawerApi.open();
}

async function handleDelete(row: Action) {
  if (!row.id) return;
  try {
    await actionStore.deleteAction(row.id);
    notification.success({
      message: $t('executor.page.action.deleteSuccess'),
    });
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('executor.page.action.title')">
      <template #toolbar-tools>
        <Button
          class="mr-2"
          type="primary"
          :icon="h(LucidePlus)"
          @click="handleCreate"
        >
          {{ $t('executor.page.action.create') }}
        </Button>
      </template>
      <template #using="{ row }">
        <Tag :color="usingToColor(row.using)">{{ row.using || '-' }}</Tag>
      </template>
      <template #enabled="{ row }">
        <Tag :color="enableBoolToColor(row.enabled)">
          {{ enableBoolToName(row.enabled) }}
        </Tag>
      </template>
      <template #action="{ row }">
        <Space>
          <Button
            type="link"
            size="small"
            :icon="h(LucideEye)"
            :title="$t('ui.button.view')"
            @click.stop="handleView(row)"
          />
          <Button
            type="link"
            size="small"
            :icon="h(LucidePencil)"
            :title="$t('executor.page.action.edit')"
            @click.stop="handleEdit(row)"
          />
          <a-popconfirm
            :cancel-text="$t('ui.button.cancel')"
            :ok-text="$t('ui.button.ok')"
            :title="$t('executor.page.action.confirmDelete')"
            @confirm="handleDelete(row)"
          >
            <Button
              danger
              type="link"
              size="small"
              :icon="h(LucideTrash2)"
              :title="$t('executor.page.action.delete')"
            />
          </a-popconfirm>
        </Space>
      </template>
    </Grid>

    <ActionDrawerComponent />
  </Page>
</template>
