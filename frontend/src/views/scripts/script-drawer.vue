<script lang="ts" setup>
import { ref, computed, onBeforeUnmount, nextTick, watch } from 'vue';

import { useVbenDrawer } from 'shell/vben/common-ui';

import {
  Form,
  FormItem,
  Input,
  InputPassword,
  Button,
  notification,
  Select,
  Switch,
  Descriptions,
  DescriptionsItem,
  Tag,
} from 'ant-design-vue';

import { $t } from 'shell/locales';
import { useExecutorScriptStore } from '../../stores/executor-script.state';
import type {
  Script,
  ScriptType,
} from '../../api/services';

const scriptStore = useExecutorScriptStore();

const data = ref<{
  mode: 'create' | 'edit' | 'view';
  row?: Script;
}>();
const loading = ref(false);

const editorContainer = ref<HTMLElement>();
let monacoEditor: any = null;
let monacoModule: any = null;

const formState = ref<{
  name: string;
  description: string;
  scriptType: ScriptType;
  content: string;
  enabled: boolean;
  password: string;
}>({
  name: '',
  description: '',
  scriptType: 'SCRIPT_TYPE_BASH',
  content: '',
  enabled: true,
  password: '',
});

const scriptTypeOptions = computed(() => [
  { value: 'SCRIPT_TYPE_BASH', label: $t('executor.page.script.typeBash') },
  {
    value: 'SCRIPT_TYPE_JAVASCRIPT',
    label: $t('executor.page.script.typeJavascript'),
  },
  { value: 'SCRIPT_TYPE_LUA', label: $t('executor.page.script.typeLua') },
]);

function scriptTypeToMonacoLang(type: ScriptType | string): string {
  switch (type) {
    case 'SCRIPT_TYPE_BASH':
      return 'shell';
    case 'SCRIPT_TYPE_JAVASCRIPT':
      return 'javascript';
    case 'SCRIPT_TYPE_LUA':
      return 'lua';
    default:
      return 'plaintext';
  }
}

function scriptTypeToName(type: string | undefined) {
  const option = scriptTypeOptions.value.find((o) => o.value === type);
  return option?.label ?? type ?? '';
}

const title = computed(() => {
  switch (data.value?.mode) {
    case 'create':
      return $t('executor.page.script.create');
    case 'edit':
      return $t('executor.page.script.edit');
    default:
      return $t('executor.page.script.view');
  }
});

const isCreateMode = computed(() => data.value?.mode === 'create');
const isEditMode = computed(() => data.value?.mode === 'edit');
const isViewMode = computed(() => data.value?.mode === 'view');

async function initMonaco() {
  if (!editorContainer.value) return;

  if (!monacoModule) {
    monacoModule = await import('monaco-editor');
  }

  if (monacoEditor) {
    monacoEditor.dispose();
    monacoEditor = null;
  }

  const lang = scriptTypeToMonacoLang(formState.value.scriptType);
  monacoEditor = monacoModule.editor.create(editorContainer.value, {
    value: formState.value.content,
    language: lang,
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

watch(
  () => formState.value.scriptType,
  (newType) => {
    if (monacoEditor && monacoModule) {
      const model = monacoEditor.getModel();
      if (model) {
        monacoModule.editor.setModelLanguage(
          model,
          scriptTypeToMonacoLang(newType),
        );
      }
    }
  },
);

function resetForm() {
  formState.value = {
    name: '',
    description: '',
    scriptType: 'SCRIPT_TYPE_BASH',
    content: '',
    enabled: true,
    password: '',
  };
}

async function handleSubmit() {
  loading.value = true;
  try {
    if (isCreateMode.value) {
      await scriptStore.createScript({
        name: formState.value.name,
        description: formState.value.description || undefined,
        scriptType: formState.value.scriptType,
        content: formState.value.content,
        enabled: formState.value.enabled,
      });
      notification.success({
        message: $t('executor.page.script.createSuccess'),
      });
    } else if (isEditMode.value && data.value?.row) {
      const updateData: Record<string, unknown> = {
        name: formState.value.name,
        description: formState.value.description,
        enabled: formState.value.enabled,
      };

      if (formState.value.content !== data.value.row.content) {
        updateData.content = formState.value.content;
        updateData.password = formState.value.password;
      }

      await scriptStore.updateScript(data.value.row.id, updateData);
      notification.success({
        message: $t('executor.page.script.updateSuccess'),
      });
    }
    drawerApi.close();
  } catch (e) {
    console.error('Failed to save script:', e);
    notification.error({
      message: $t('ui.notification.create_failed'),
    });
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
        row?: Script;
      };

      if (data.value?.mode === 'create') {
        resetForm();
      } else if (data.value?.row) {
        formState.value = {
          name: data.value.row.name,
          description: data.value.row.description ?? '',
          scriptType: data.value.row.scriptType,
          content: data.value.row.content ?? '',
          enabled: data.value.row.enabled,
          password: '',
        };
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
  <Drawer :title="title" :footer="false">
    <!-- View Mode -->
    <template v-if="data?.row && isViewMode">
      <Descriptions :column="1" bordered size="small">
        <DescriptionsItem :label="$t('executor.page.script.name')">
          {{ data.row.name }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.script.description')">
          {{ data.row.description || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.script.scriptType')">
          <Tag>{{ scriptTypeToName(data.row.scriptType) }}</Tag>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.script.version')">
          {{ data.row.version }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.script.contentHash')">
          <span class="font-mono text-xs">{{ data.row.contentHash }}</span>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.script.enabled')">
          <Tag :color="data.row.enabled ? '#52C41A' : '#8C8C8C'">
            {{ data.row.enabled ? 'Enabled' : 'Disabled' }}
          </Tag>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.script.createdAt')">
          {{ data.row.createTime || '-' }}
        </DescriptionsItem>
      </Descriptions>

      <div class="mt-4">
        <h4 class="mb-2 text-base font-medium">
          {{ $t('executor.page.script.content') }}
        </h4>
        <div
          ref="editorContainer"
          style="height: 400px; border: 1px solid #d9d9d9; border-radius: 4px"
        />
      </div>
    </template>

    <!-- Create / Edit Mode -->
    <template v-else-if="isCreateMode || isEditMode">
      <Form layout="vertical" :model="formState" @finish="handleSubmit">
        <FormItem
          :label="$t('executor.page.script.name')"
          name="name"
          :rules="[{ required: true, message: $t('ui.formRules.required') }]"
        >
          <Input
            v-model:value="formState.name"
            :placeholder="$t('executor.page.script.namePlaceholder')"
          />
        </FormItem>

        <FormItem
          :label="$t('executor.page.script.description')"
          name="description"
        >
          <Input
            v-model:value="formState.description"
            :placeholder="$t('executor.page.script.descriptionPlaceholder')"
          />
        </FormItem>

        <FormItem
          :label="$t('executor.page.script.scriptType')"
          name="scriptType"
          :rules="[{ required: true, message: $t('ui.formRules.required') }]"
        >
          <Select
            v-model:value="formState.scriptType"
            :options="scriptTypeOptions"
            :placeholder="$t('executor.page.script.selectType')"
            :disabled="isEditMode"
          />
        </FormItem>

        <FormItem :label="$t('executor.page.script.enabled')" name="enabled">
          <Switch v-model:checked="formState.enabled" />
        </FormItem>

        <FormItem
          :label="$t('executor.page.script.content')"
          :rules="[{ required: true, message: $t('ui.formRules.required') }]"
        >
          <div
            ref="editorContainer"
            style="
              height: 350px;
              border: 1px solid #d9d9d9;
              border-radius: 4px;
            "
          />
        </FormItem>

        <FormItem
          v-if="isEditMode"
          :label="$t('executor.page.script.passwordRequired')"
          name="password"
        >
          <InputPassword
            v-model:value="formState.password"
            :placeholder="$t('executor.page.script.passwordPlaceholder')"
          />
        </FormItem>

        <FormItem>
          <Button type="primary" html-type="submit" :loading="loading" block>
            {{
              isCreateMode
                ? $t('executor.page.script.create')
                : $t('executor.page.script.edit')
            }}
          </Button>
        </FormItem>
      </Form>
    </template>
  </Drawer>
</template>
