<script lang="ts" setup>
// Settings handed to go-tangra-client so it can install the go-tangra v4
// inventory agent and enroll it with an auto-enrollment key. The key secret
// is write-only: it is sent when changed and never shown again.
import { reactive, ref, watch } from 'vue';

import {
  Alert,
  Button,
  Checkbox,
  Drawer,
  Form,
  FormItem,
  Input,
  InputPassword,
  Switch,
  Textarea,
  notification,
} from 'ant-design-vue';

import { $t } from 'shell/locales';
import { InventoryAgentSettingsService, type InventoryAgentSettings } from '../../api/services';

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ (e: 'update:open', v: boolean): void }>();

const loading = ref(false);
const saving = ref(false);
const current = ref<InventoryAgentSettings | null>(null);
const form = reactive({
  enabled: false,
  ingestEndpoint: '',
  keyId: '',
  keySecret: '',
  clearKeySecret: false,
  caPem: '',
  serverName: '',
  agentVersion: 'latest',
});

async function load() {
  loading.value = true;
  try {
    const s = await InventoryAgentSettingsService.get();
    current.value = s;
    Object.assign(form, {
      enabled: !!s.enabled,
      ingestEndpoint: s.ingestEndpoint ?? '',
      keyId: s.keyId ?? '',
      keySecret: '',
      clearKeySecret: false,
      caPem: s.caPem ?? '',
      serverName: s.serverName ?? '',
      agentVersion: s.agentVersion || 'latest',
    });
  } catch (e) {
    notification.error({ message: $t('executor.page.inventoryAgent.loadFailed'), description: (e as Error).message });
  } finally {
    loading.value = false;
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) void load();
  },
  { immediate: true },
);

async function save() {
  saving.value = true;
  try {
    current.value = await InventoryAgentSettingsService.update({
      enabled: form.enabled,
      ingestEndpoint: form.ingestEndpoint.trim(),
      keyId: form.keyId.trim(),
      keySecret: form.keySecret.trim() || undefined,
      clearKeySecret: form.clearKeySecret,
      caPem: form.caPem.trim(),
      serverName: form.serverName.trim(),
      agentVersion: form.agentVersion.trim() === 'latest' ? '' : form.agentVersion.trim(),
    });
    form.keySecret = '';
    form.clearKeySecret = false;
    notification.success({ message: $t('executor.page.inventoryAgent.saved') });
    emit('update:open', false);
  } catch (e) {
    notification.error({ message: $t('executor.page.inventoryAgent.saveFailed'), description: (e as Error).message });
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <Drawer
    :open="open"
    :title="$t('executor.page.inventoryAgent.title')"
    width="560"
    @close="emit('update:open', false)"
  >
    <Alert type="info" show-icon class="mb-4" :message="$t('executor.page.inventoryAgent.intro')" />
    <Form layout="vertical" :disabled="loading">
      <FormItem>
        <Switch v-model:checked="form.enabled" />
        <span class="ml-2">{{ $t('executor.page.inventoryAgent.enabled') }}</span>
      </FormItem>
      <FormItem :label="$t('executor.page.inventoryAgent.ingestEndpoint')" :extra="$t('executor.page.inventoryAgent.ingestEndpointHint')">
        <Input v-model:value="form.ingestEndpoint" placeholder="portal.example.org:9977" />
      </FormItem>
      <FormItem :label="$t('executor.page.inventoryAgent.keyId')">
        <Input v-model:value="form.keyId" placeholder="ak_..." class="font-mono" />
      </FormItem>
      <FormItem
        :label="$t('executor.page.inventoryAgent.keySecret')"
        :extra="current?.keySecretConfigured ? $t('executor.page.inventoryAgent.keySecretConfigured') : $t('executor.page.inventoryAgent.keySecretMissing')"
      >
        <InputPassword v-model:value="form.keySecret" placeholder="aks_..." autocomplete="new-password" :disabled="form.clearKeySecret" />
        <Checkbox v-if="current?.keySecretConfigured" v-model:checked="form.clearKeySecret" class="mt-2">
          {{ $t('executor.page.inventoryAgent.clearKeySecret') }}
        </Checkbox>
      </FormItem>
      <FormItem :label="$t('executor.page.inventoryAgent.caPem')" :extra="$t('executor.page.inventoryAgent.caPemHint')">
        <Textarea v-model:value="form.caPem" :rows="4" class="font-mono text-xs" placeholder="-----BEGIN CERTIFICATE-----" />
      </FormItem>
      <FormItem :label="$t('executor.page.inventoryAgent.serverName')">
        <Input v-model:value="form.serverName" />
      </FormItem>
      <FormItem :label="$t('executor.page.inventoryAgent.agentVersion')" :extra="$t('executor.page.inventoryAgent.agentVersionHint')">
        <Input v-model:value="form.agentVersion" placeholder="latest" />
      </FormItem>
      <div v-if="current?.updateTime" class="text-xs text-gray-400">
        {{ $t('executor.page.inventoryAgent.updated') }}: {{ new Date(current.updateTime).toLocaleString() }}
      </div>
    </Form>
    <template #footer>
      <div class="flex justify-end">
        <Button type="primary" :loading="saving" @click="save">
          {{ $t('executor.page.inventoryAgent.save') }}
        </Button>
      </div>
    </template>
  </Drawer>
</template>
