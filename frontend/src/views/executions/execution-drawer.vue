<script lang="ts" setup>
import { ref, computed } from 'vue';

import { useVbenDrawer } from 'shell/vben/common-ui';

import {
  Descriptions,
  DescriptionsItem,
  Tag,
  Divider,
  Spin,
} from 'ant-design-vue';

import { $t } from 'shell/locales';
import { useExecutorExecutionStore } from '../../stores/executor-execution.state';
import type {
  ExecutionLog,
  GetExecutionOutputResponse,
} from '../../api/services';

const executionStore = useExecutorExecutionStore();

const execution = ref<ExecutionLog>();
const output = ref<GetExecutionOutputResponse>();
const outputLoading = ref(false);

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

function formatDuration(ms: number | undefined) {
  if (ms === undefined || ms === null) return '-';
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

async function loadOutput(id: string) {
  outputLoading.value = true;
  try {
    output.value = await executionStore.getExecutionOutput(id);
  } catch (e) {
    console.error('Failed to load execution output:', e);
    output.value = undefined;
  } finally {
    outputLoading.value = false;
  }
}

const [Drawer, drawerApi] = useVbenDrawer({
  onCancel() {
    drawerApi.close();
  },

  async onOpenChange(isOpen) {
    if (isOpen) {
      const drawerData = drawerApi.getData() as { row: ExecutionLog };
      execution.value = drawerData.row;
      output.value = undefined;
      if (execution.value?.id) {
        await loadOutput(execution.value.id);
      }
    }
  },
});
</script>

<template>
  <Drawer :title="$t('executor.page.execution.view')" :footer="false">
    <template v-if="execution">
      <Descriptions :column="1" bordered size="small">
        <DescriptionsItem :label="$t('executor.page.execution.scriptName')">
          {{ execution.scriptName || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.execution.clientId')">
          <span class="font-mono text-xs">{{ execution.clientId }}</span>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.execution.triggerType')">
          <Tag>{{ triggerTypeToName(execution.triggerType) }}</Tag>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.execution.status')">
          <Tag :color="statusToColor(execution.status)">
            {{ statusToName(execution.status) }}
          </Tag>
        </DescriptionsItem>
        <DescriptionsItem
          v-if="execution.exitCode !== undefined && execution.exitCode !== null"
          :label="$t('executor.page.execution.exitCode')"
        >
          <Tag :color="execution.exitCode === 0 ? '#52C41A' : '#FF4D4F'">
            {{ execution.exitCode }}
          </Tag>
        </DescriptionsItem>
        <DescriptionsItem
          v-if="execution.durationMs"
          :label="$t('executor.page.execution.duration')"
        >
          {{ formatDuration(execution.durationMs) }}
        </DescriptionsItem>
        <DescriptionsItem
          v-if="execution.rejectionReason"
          :label="$t('executor.page.execution.rejectionReason')"
        >
          {{ execution.rejectionReason }}
        </DescriptionsItem>
        <DescriptionsItem
          v-if="execution.startedAt"
          :label="$t('executor.page.execution.startedAt')"
        >
          {{ execution.startedAt }}
        </DescriptionsItem>
        <DescriptionsItem
          v-if="execution.completedAt"
          :label="$t('executor.page.execution.completedAt')"
        >
          {{ execution.completedAt }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('executor.page.execution.createdAt')">
          {{ execution.createTime || '-' }}
        </DescriptionsItem>
      </Descriptions>

      <!-- Output Section -->
      <Divider />
      <Spin :spinning="outputLoading">
        <div v-if="output">
          <div v-if="output.output" class="mb-4">
            <h4 class="mb-2 text-base font-medium">
              {{ $t('executor.page.execution.output') }}
            </h4>
            <pre
              class="max-h-64 overflow-auto rounded bg-gray-900 p-3 font-mono text-xs text-green-400"
            >{{ output.output }}</pre>
          </div>
          <div v-if="output.errorOutput">
            <h4 class="mb-2 text-base font-medium">
              {{ $t('executor.page.execution.errorOutput') }}
            </h4>
            <pre
              class="max-h-64 overflow-auto rounded bg-gray-900 p-3 font-mono text-xs text-red-400"
            >{{ output.errorOutput }}</pre>
          </div>
        </div>
      </Spin>
    </template>
  </Drawer>
</template>
