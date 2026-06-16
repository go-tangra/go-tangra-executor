import { defineStore } from 'pinia';

import {
  ConnectedClientsService,
  WorkflowRunService,
  WorkflowService,
  type CreateWorkflowRequest,
  type ListWorkflowsResponse,
  type TriggerWorkflowRequest,
  type UpdateWorkflowRequest,
  type Workflow,
} from '../api/services';

export const useExecutorWorkflowStore = defineStore('executor-workflow', () => {
  async function listWorkflows(
    paging?: { page?: number; pageSize?: number },
    filters?: { name?: string; enabled?: boolean } | null,
  ): Promise<ListWorkflowsResponse> {
    return await WorkflowService.list({
      page: paging?.page,
      pageSize: paging?.pageSize,
      name: filters?.name,
      enabled: filters?.enabled,
    });
  }

  async function getWorkflow(id: string): Promise<{ workflow: Workflow }> {
    return await WorkflowService.get(id);
  }

  async function createWorkflow(
    data: CreateWorkflowRequest,
  ): Promise<{ workflow: Workflow }> {
    return await WorkflowService.create(data);
  }

  async function updateWorkflow(
    id: string,
    data: UpdateWorkflowRequest,
  ): Promise<{ workflow: Workflow }> {
    return await WorkflowService.update(id, data);
  }

  async function deleteWorkflow(id: string): Promise<void> {
    return await WorkflowService.delete(id);
  }

  async function listConnectedClients() {
    return await ConnectedClientsService.list();
  }

  async function runWorkflow(clientId: string, data: TriggerWorkflowRequest) {
    return await WorkflowRunService.run(clientId, data);
  }

  function $reset() {}

  return {
    $reset,
    listWorkflows,
    getWorkflow,
    createWorkflow,
    updateWorkflow,
    deleteWorkflow,
    listConnectedClients,
    runWorkflow,
  };
});
