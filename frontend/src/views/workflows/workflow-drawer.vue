<script lang="ts" setup>
import { ref, computed, onBeforeUnmount, nextTick } from 'vue';

import { useVbenDrawer } from 'shell/vben/common-ui';

import {
  Form,
  FormItem,
  Input,
  Button,
  notification,
  Switch,
  Select,
  Descriptions,
  DescriptionsItem,
} from 'ant-design-vue';

import { $t } from 'shell/locales';
import { useExecutorWorkflowStore } from '../../stores/executor-workflow.state';
import { ActionService, type Workflow } from '../../api/services';

const workflowStore = useExecutorWorkflowStore();

const data = ref<{ mode: 'create' | 'edit' | 'view'; row?: Workflow }>();
const loading = ref(false);

const editorContainer = ref<HTMLElement>();
let monacoEditor: any = null;
let monacoModule: any = null;

const formState = ref<{ name: string; description: string; enabled: boolean; content: string }>({
  name: '',
  description: '',
  enabled: true,
  content: '',
});

const actionOptions = ref<{ value: string; label: string }[]>([]);

const DEFAULT_CONTENT = `name: my-workflow
jobs:
  main:
    steps:
      - name: Hello
        run: echo "hello from $(hostname)"
        shell: bash
`;

const title = computed(() => {
  switch (data.value?.mode) {
    case 'create':
      return $t('executor.page.workflow.create');
    case 'edit':
      return $t('executor.page.workflow.edit');
    default:
      return $t('executor.page.workflow.view');
  }
});

const isCreateMode = computed(() => data.value?.mode === 'create');
const isEditMode = computed(() => data.value?.mode === 'edit');
const isViewMode = computed(() => data.value?.mode === 'view');

async function loadActions() {
  try {
    const resp = await ActionService.list({ pageSize: 200, enabled: true });
    actionOptions.value = (resp.actions ?? []).map((a) => ({
      value: a.name,
      label: `${a.name}${a.using ? ` (${a.using})` : ''}`,
    }));
  } catch {
    actionOptions.value = [];
  }
}

// Append a `uses:` step for the chosen repo action into the editor.
function insertAction(name: unknown) {
  const actionName = String(name ?? '');
  if (!actionName || !monacoEditor) return;
  const snippet = `      - name: ${actionName}\n        uses: ${actionName}\n`;
  const current = monacoEditor.getValue();
  const next = current.endsWith('\n') ? current + snippet : current + '\n' + snippet;
  monacoEditor.setValue(next);
  formState.value.content = next;
}

async function initMonaco() {
  if (!editorContainer.value) return;
  if (!monacoModule) monacoModule = await import('monaco-editor');
  if (monacoEditor) {
    monacoEditor.dispose();
    monacoEditor = null;
  }
  monacoEditor = monacoModule.editor.create(editorContainer.value, {
    value: formState.value.content,
    language: 'yaml',
    theme: 'vs-dark',
    readOnly: isViewMode.value,
    minimap: { enabled: false },
    scrollBeyondLastLine: false,
    fontSize: 13,
    lineNumbers: 'on',
    automaticLayout: true,
    tabSize: 2,
    wordWrap: 'on',
  });
  if (!isViewMode.value) {
    monacoEditor.onDidChangeModelContent(() => {
      formState.value.content = monacoEditor.getValue();
    });
  }
}

function resetForm() {
  formState.value = { name: '', description: '', enabled: true, content: DEFAULT_CONTENT };
}

async function handleSubmit() {
  loading.value = true;
  try {
    if (isCreateMode.value) {
      await workflowStore.createWorkflow({
        name: formState.value.name,
        description: formState.value.description || undefined,
        content: formState.value.content,
        enabled: formState.value.enabled,
      });
      notification.success({ message: $t('executor.page.workflow.createSuccess') });
    } else if (isEditMode.value && data.value?.row) {
      await workflowStore.updateWorkflow(data.value.row.id, {
        name: formState.value.name,
        description: formState.value.description,
        content: formState.value.content,
        enabled: formState.value.enabled,
      });
      notification.success({ message: $t('executor.page.workflow.updateSuccess') });
    }
    drawerApi.close();
  } catch (e) {
    console.error('Failed to save workflow:', e);
    notification.error({ message: $t('ui.notification.create_failed') });
  } finally {
    loading.value = false;
  }
}

const [Drawer, drawerApi] = useVbenDrawer({
  onCancel() {
    drawerApi.close();
  },
  async onOpenChange(isOpen) {
    if (isOpen) {
      data.value = drawerApi.getData() as { mode: 'create' | 'edit' | 'view'; row?: Workflow };
      if (data.value?.mode === 'create') {
        resetForm();
      } else if (data.value?.row) {
        formState.value = {
          name: data.value.row.name,
          description: data.value.row.description ?? '',
          enabled: data.value.row.enabled,
          content: data.value.row.content ?? '',
        };
      }
      if (!isViewMode.value) loadActions();
      await nextTick();
      await initMonaco();
    } else if (monacoEditor) {
      monacoEditor.dispose();
      monacoEditor = null;
    }
  },
});

onBeforeUnmount(() => {
  if (monacoEditor) {
    monacoEditor.dispose();
    monacoEditor = null;
  }
});
</script>

<template>
  <Drawer :title="title" :footer="false" class="w-full max-w-3xl">
    <template v-if="data?.row && isViewMode">
      <Descriptions :column="1" bordered size="small">
        <DescriptionsItem :label="$t('executor.page.workflow.name')">{{ data.row.name }}</DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.workflow.description')">{{ data.row.description || '-' }}</DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.workflow.version')">{{ data.row.version }}</DescriptionsItem>
      </Descriptions>
      <div class="mt-4">
        <h4 class="mb-2 text-base font-medium">{{ $t('executor.page.workflow.content') }}</h4>
        <div ref="editorContainer" style="height: 380px; border: 1px solid #d9d9d9; border-radius: 4px" />
      </div>
    </template>

    <template v-else-if="isCreateMode || isEditMode">
      <Form layout="vertical" :model="formState" @finish="handleSubmit">
        <FormItem
          :label="$t('executor.page.workflow.name')"
          name="name"
          :rules="[{ required: true, message: $t('ui.formRules.required') }]"
        >
          <Input v-model:value="formState.name" :placeholder="$t('executor.page.workflow.namePlaceholder')" />
        </FormItem>

        <FormItem :label="$t('executor.page.workflow.description')" name="description">
          <Input v-model:value="formState.description" :placeholder="$t('executor.page.workflow.descriptionPlaceholder')" />
        </FormItem>

        <FormItem :label="$t('executor.page.workflow.enabled')" name="enabled">
          <Switch v-model:checked="formState.enabled" />
        </FormItem>

        <FormItem :label="$t('executor.page.workflow.content')">
          <div class="mb-2">
            <Select
              :options="actionOptions"
              :placeholder="$t('executor.page.workflow.insertAction')"
              show-search
              style="width: 100%"
              :value="undefined"
              @change="insertAction"
            />
          </div>
          <div ref="editorContainer" style="height: 340px; border: 1px solid #d9d9d9; border-radius: 4px" />
        </FormItem>

        <FormItem>
          <Button type="primary" html-type="submit" :loading="loading" block>
            {{ isCreateMode ? $t('executor.page.workflow.create') : $t('executor.page.workflow.edit') }}
          </Button>
        </FormItem>
      </Form>
    </template>
  </Drawer>
</template>
