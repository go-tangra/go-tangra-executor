<script lang="ts" setup>
import { ref, computed, onBeforeUnmount, nextTick } from 'vue';

import { useVbenDrawer } from 'shell/vben/common-ui';

import {
  Form,
  FormItem,
  Input,
  InputPassword,
  Textarea,
  Button,
  notification,
  Switch,
  Descriptions,
  DescriptionsItem,
  Tag,
} from 'ant-design-vue';

import { $t } from 'shell/locales';
import { useExecutorActionStore } from '../../stores/executor-action.state';
import type { Action, ActionFile } from '../../api/services';

const actionStore = useExecutorActionStore();

const data = ref<{
  mode: 'create' | 'edit' | 'view';
  row?: Action;
}>();
const loading = ref(false);

const editorContainer = ref<HTMLElement>();
let monacoEditor: any = null;
let monacoModule: any = null;

const formState = ref<{
  name: string;
  description: string;
  enabled: boolean;
  manifest: string;
  files: ActionFile[];
  password: string;
}>({
  name: '',
  description: '',
  enabled: true,
  manifest: '',
  files: [],
  password: '',
});

let originalManifest = '';
let originalFilesJson = '[]';

const DEFAULT_MANIFEST = `name: my-action
description: ""
runs:
  using: composite
  steps: []
`;

const title = computed(() => {
  switch (data.value?.mode) {
    case 'create':
      return $t('executor.page.action.create');
    case 'edit':
      return $t('executor.page.action.edit');
    default:
      return $t('executor.page.action.view');
  }
});

const isCreateMode = computed(() => data.value?.mode === 'create');
const isEditMode = computed(() => data.value?.mode === 'edit');
const isViewMode = computed(() => data.value?.mode === 'view');

const contentDirty = computed(
  () =>
    formState.value.manifest !== originalManifest ||
    JSON.stringify(formState.value.files) !== originalFilesJson,
);

async function initMonaco() {
  if (!editorContainer.value) return;

  if (!monacoModule) {
    monacoModule = await import('monaco-editor');
  }
  if (monacoEditor) {
    monacoEditor.dispose();
    monacoEditor = null;
  }

  monacoEditor = monacoModule.editor.create(editorContainer.value, {
    value: formState.value.manifest,
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
      formState.value.manifest = monacoEditor.getValue();
    });
  }
}

function resetForm() {
  formState.value = {
    name: '',
    description: '',
    enabled: true,
    manifest: DEFAULT_MANIFEST,
    files: [],
    password: '',
  };
  originalManifest = '';
  originalFilesJson = '[]';
}

function addFile() {
  formState.value.files.push({ path: '', content: '' });
}

function removeFile(idx: number) {
  formState.value.files.splice(idx, 1);
}

async function handleSubmit() {
  loading.value = true;
  try {
    const files = formState.value.files.filter((f) => f.path.trim() !== '');

    if (isCreateMode.value) {
      await actionStore.createAction({
        name: formState.value.name,
        description: formState.value.description || undefined,
        manifest: formState.value.manifest,
        files,
        enabled: formState.value.enabled,
      });
      notification.success({
        message: $t('executor.page.action.createSuccess'),
      });
    } else if (isEditMode.value && data.value?.row) {
      const updateData: Record<string, unknown> = {
        description: formState.value.description,
        enabled: formState.value.enabled,
      };
      // Only send manifest+files when content actually changed; their presence
      // tells the backend to replace the package and bump the version.
      if (contentDirty.value) {
        updateData.manifest = formState.value.manifest;
        updateData.files = files;
        updateData.password = formState.value.password;
      }
      await actionStore.updateAction(data.value.row.id, updateData);
      notification.success({
        message: $t('executor.page.action.updateSuccess'),
      });
    }
    drawerApi.close();
  } catch (e) {
    console.error('Failed to save action:', e);
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
      data.value = drawerApi.getData() as {
        mode: 'create' | 'edit' | 'view';
        row?: Action;
      };

      if (data.value?.mode === 'create') {
        resetForm();
      } else if (data.value?.row) {
        const files = (data.value.row.files ?? []).map((f) => ({
          path: f.path,
          content: f.content,
        }));
        formState.value = {
          name: data.value.row.name,
          description: data.value.row.description ?? '',
          enabled: data.value.row.enabled,
          manifest: data.value.row.manifest ?? '',
          files,
          password: '',
        };
        originalManifest = formState.value.manifest;
        originalFilesJson = JSON.stringify(files);
      }

      await nextTick();
      await initMonaco();
    } else {
      if (monacoEditor) {
        monacoEditor.dispose();
        monacoEditor = null;
      }
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
    <!-- View Mode -->
    <template v-if="data?.row && isViewMode">
      <Descriptions :column="1" bordered size="small">
        <DescriptionsItem :label="$t('executor.page.action.name')">
          {{ data.row.name }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.action.description')">
          {{ data.row.description || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.action.using')">
          <Tag>{{ data.row.using || '-' }}</Tag>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.action.version')">
          {{ data.row.version }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.action.enabled')">
          <Tag :color="data.row.enabled ? '#52C41A' : '#8C8C8C'">
            {{ data.row.enabled ? 'Enabled' : 'Disabled' }}
          </Tag>
        </DescriptionsItem>
      </Descriptions>

      <div class="mt-4">
        <h4 class="mb-2 text-base font-medium">
          {{ $t('executor.page.action.manifest') }}
        </h4>
        <div
          ref="editorContainer"
          style="height: 320px; border: 1px solid #d9d9d9; border-radius: 4px"
        />
      </div>

      <div v-if="(data.row.files ?? []).length" class="mt-4">
        <h4 class="mb-2 text-base font-medium">
          {{ $t('executor.page.action.files') }}
        </h4>
        <div v-for="(f, i) in data.row.files" :key="i" class="mb-3">
          <div class="mb-1 font-mono text-xs">{{ f.path }}</div>
          <Textarea
            :value="f.content"
            readonly
            :auto-size="{ minRows: 3, maxRows: 12 }"
            style="font-family: monospace; font-size: 12px"
          />
        </div>
      </div>
    </template>

    <!-- Create / Edit Mode -->
    <template v-else-if="isCreateMode || isEditMode">
      <Form layout="vertical" :model="formState" @finish="handleSubmit">
        <FormItem
          :label="$t('executor.page.action.name')"
          name="name"
          :rules="[{ required: true, message: $t('ui.formRules.required') }]"
        >
          <Input
            v-model:value="formState.name"
            :disabled="isEditMode"
            :placeholder="$t('executor.page.action.namePlaceholder')"
          />
        </FormItem>

        <FormItem
          :label="$t('executor.page.action.description')"
          name="description"
        >
          <Input
            v-model:value="formState.description"
            :placeholder="$t('executor.page.action.descriptionPlaceholder')"
          />
        </FormItem>

        <FormItem :label="$t('executor.page.action.enabled')" name="enabled">
          <Switch v-model:checked="formState.enabled" />
        </FormItem>

        <FormItem
          :label="$t('executor.page.action.manifest')"
          :rules="[{ required: true, message: $t('ui.formRules.required') }]"
        >
          <div
            ref="editorContainer"
            style="height: 300px; border: 1px solid #d9d9d9; border-radius: 4px"
          />
        </FormItem>

        <FormItem :label="$t('executor.page.action.files')">
          <div
            v-for="(f, i) in formState.files"
            :key="i"
            class="mb-3 rounded border border-gray-200 p-2"
          >
            <div class="mb-1 flex gap-2">
              <Input
                v-model:value="f.path"
                :placeholder="$t('executor.page.action.filePathPlaceholder')"
                style="flex: 1"
              />
              <Button danger size="small" @click="removeFile(i)">
                {{ $t('ui.button.delete') }}
              </Button>
            </div>
            <Textarea
              v-model:value="f.content"
              :auto-size="{ minRows: 3, maxRows: 16 }"
              :placeholder="$t('executor.page.action.fileContentPlaceholder')"
              style="font-family: monospace; font-size: 12px"
            />
          </div>
          <Button type="dashed" block @click="addFile">
            {{ $t('executor.page.action.addFile') }}
          </Button>
        </FormItem>

        <FormItem
          v-if="isEditMode && contentDirty"
          :label="$t('executor.page.action.passwordRequired')"
          name="password"
        >
          <InputPassword
            v-model:value="formState.password"
            :placeholder="$t('executor.page.action.passwordPlaceholder')"
          />
        </FormItem>

        <FormItem>
          <Button type="primary" html-type="submit" :loading="loading" block>
            {{
              isCreateMode
                ? $t('executor.page.action.create')
                : $t('executor.page.action.edit')
            }}
          </Button>
        </FormItem>
      </Form>
    </template>
  </Drawer>
</template>
