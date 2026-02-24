import { defineStore } from 'pinia';

import {
  ExecutionService,
  type ExecutionLog,
  type GetExecutionOutputResponse,
  type ListExecutionsResponse,
} from '../api/services';

export const useExecutorExecutionStore = defineStore(
  'executor-execution',
  () => {
    async function triggerExecution(
      scriptId: string,
      clientId: string,
    ): Promise<{ execution: ExecutionLog }> {
      return await ExecutionService.trigger(scriptId, clientId);
    }

    async function getExecution(
      id: string,
    ): Promise<{ execution: ExecutionLog }> {
      return await ExecutionService.get(id);
    }

    async function listExecutions(
      paging?: { page?: number; pageSize?: number },
      filters?: {
        scriptId?: string;
        clientId?: string;
        status?: string;
      } | null,
    ): Promise<ListExecutionsResponse> {
      return await ExecutionService.list({
        page: paging?.page,
        pageSize: paging?.pageSize,
        scriptId: filters?.scriptId,
        clientId: filters?.clientId,
        status: filters?.status,
      });
    }

    async function getExecutionOutput(
      id: string,
    ): Promise<GetExecutionOutputResponse> {
      return await ExecutionService.getOutput(id);
    }

    function $reset() {}

    return {
      $reset,
      triggerExecution,
      getExecution,
      listExecutions,
      getExecutionOutput,
    };
  },
);
