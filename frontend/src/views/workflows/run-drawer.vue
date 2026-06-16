<script lang="ts" setup>
import { ref, computed, onBeforeUnmount, nextTick, h } from 'vue';

import { useVbenDrawer } from 'shell/vben/common-ui';

import {
  Form,
  FormItem,
  Button,
  Input,
  Select,
  Tag,
  notification,
  Empty,
} from 'ant-design-vue';
import { LucidePlus, LucideTrash2 } from 'shell/vben/icons';

import { $t } from 'shell/locales';
import { useExecutorWorkflowStore } from '../../stores/executor-workflow.state';
import { ExecutionService, type Workflow } from '../../api/services';

const workflowStore = useExecutorWorkflowStore();

const data = ref<{ workflow?: Workflow }>();
const phase = ref<'form' | 'running'>('form');
const loading = ref(false);

const clientOptions = ref<{ value: string; label: string }[]>([]);
const selectedClient = ref<string>('');
const inputs = ref<{ key: string; value: string }[]>([]);

const execId = ref('');
const status = ref('');
const exitCode = ref<number | undefined>(undefined);
const output = ref('');
const outputBox = ref<HTMLElement>();
let pollTimer: ReturnType<typeof setInterval> | null = null;

const TERMINAL = new Set([
  'EXECUTION_STATUS_COMPLETED',
  'EXECUTION_STATUS_FAILED',
  'EXECUTION_STATUS_REJECTED_HASH_MISMATCH',
  'EXECUTION_STATUS_REJECTED_NOT_APPROVED',
  'EXECUTION_STATUS_CLIENT_OFFLINE',
]);

const statusLabel = computed(() => status.value.replace('EXECUTION_STATUS_', '') || '—');
const statusColor = computed(() => {
  switch (status.value) {
    case 'EXECUTION_STATUS_COMPLETED':
      return '#52C41A';
    case 'EXECUTION_STATUS_RUNNING':
      return '#1890FF';
    case 'EXECUTION_STATUS_PENDING':
      return '#D4B106';
    default:
      return '#CF1322';
  }
});

function addInput() {
  inputs.value.push({ key: '', value: '' });
}
function removeInput(i: number) {
  inputs.value.splice(i, 1);
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

async function poll() {
  if (!execId.value) return;
  try {
    const [exec, out] = await Promise.all([
      ExecutionService.get(execId.value),
      ExecutionService.getOutput(execId.value),
    ]);
    status.value = exec.execution?.status ?? status.value;
    exitCode.value = out.exitCode;
    output.value = out.output || '';
    await nextTick();
    if (outputBox.value) outputBox.value.scrollTop = outputBox.value.scrollHeight;
    if (TERMINAL.has(status.value)) stopPolling();
  } catch {
    // transient; keep polling
  }
}

async function handleRun() {
  if (!selectedClient.value) {
    notification.warning({ message: $t('executor.page.workflow.runClientPlaceholder') });
    return;
  }
  if (!data.value?.workflow) return;
  loading.value = true;
  try {
    const inputMap: Record<string, string> = {};
    for (const kv of inputs.value) {
      if (kv.key.trim()) inputMap[kv.key.trim()] = kv.value;
    }
    const resp = await workflowStore.runWorkflow(selectedClient.value, {
      name: data.value.workflow.name,
      workflow: data.value.workflow.content,
      inputs: inputMap,
    });
    execId.value = resp.execution?.id ?? '';
    status.value = resp.execution?.status ?? 'EXECUTION_STATUS_PENDING';
    output.value = '';
    phase.value = 'running';
    notification.success({ message: $t('executor.page.workflow.runStarted') });
    if (execId.value) {
      await poll();
      pollTimer = setInterval(poll, 1000);
    }
  } catch {
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
      data.value = drawerApi.getData() as { workflow?: Workflow };
      phase.value = 'form';
      selectedClient.value = '';
      inputs.value = [];
      execId.value = '';
      status.value = '';
      output.value = '';
      exitCode.value = undefined;
      try {
        const resp = await workflowStore.listConnectedClients();
        clientOptions.value = (resp.clients ?? []).map((c) => ({
          value: c.clientId,
          label: `${c.clientId}${c.clientVersion ? ` (v${c.clientVersion})` : ''}`,
        }));
      } catch {
        clientOptions.value = [];
      }
    } else {
      stopPolling();
    }
  },
});

onBeforeUnmount(stopPolling);
</script>

<template>
  <Drawer :title="$t('executor.page.workflow.runTitle')" :footer="false" class="w-full max-w-3xl">
    <div v-if="data?.workflow" class="mb-3">
      <Tag color="blue">{{ data.workflow.name }}</Tag>
    </div>

    <!-- Form phase -->
    <template v-if="phase === 'form'">
      <Form layout="vertical">
        <FormItem :label="$t('executor.page.workflow.runClient')" :rules="[{ required: true }]">
          <Select
            v-model:value="selectedClient"
            :options="clientOptions"
            :placeholder="$t('executor.page.workflow.runClientPlaceholder')"
            show-search
            style="width: 100%"
          />
          <div v-if="clientOptions.length === 0" class="mt-1 text-xs text-gray-400">
            {{ $t('executor.page.workflow.noClients') }}
          </div>
        </FormItem>

        <FormItem :label="$t('executor.page.workflow.runInputs')">
          <div v-for="(kv, i) in inputs" :key="i" class="mb-2 flex gap-2">
            <Input v-model:value="kv.key" placeholder="key" style="flex: 1" />
            <Input v-model:value="kv.value" placeholder="value" style="flex: 2" />
            <Button danger size="small" :icon="h(LucideTrash2)" @click="removeInput(i)" />
          </div>
          <Button type="dashed" block :icon="h(LucidePlus)" @click="addInput">
            {{ $t('executor.page.workflow.addInput') }}
          </Button>
        </FormItem>

        <Button
          type="primary"
          block
          :loading="loading"
          :disabled="!selectedClient"
          @click="handleRun"
        >
          {{ $t('executor.page.workflow.runStart') }}
        </Button>
      </Form>
    </template>

    <!-- Running / output phase -->
    <template v-else>
      <div class="mb-2 flex items-center gap-2">
        <span>{{ $t('executor.page.workflow.status') }}:</span>
        <Tag :color="statusColor">{{ statusLabel }}</Tag>
        <span v-if="exitCode !== undefined" class="text-xs text-gray-400">exit {{ exitCode }}</span>
        <span v-if="!TERMINAL.has(status)" class="text-xs text-gray-400">{{ $t('executor.page.workflow.running') }}</span>
      </div>
      <div
        ref="outputBox"
        style="
          height: 460px;
          overflow: auto;
          background: #1e1e1e;
          color: #d4d4d4;
          font-family: monospace;
          font-size: 12px;
          padding: 10px;
          border-radius: 4px;
          white-space: pre-wrap;
        "
      >
        <template v-if="output">{{ output }}</template>
        <Empty v-else :description="$t('executor.page.workflow.waiting')" />
      </div>
    </template>
  </Drawer>
</template>
