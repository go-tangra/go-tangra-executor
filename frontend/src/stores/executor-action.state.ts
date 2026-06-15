import { defineStore } from 'pinia';

import {
  ActionService,
  type Action,
  type CreateActionRequest,
  type ListActionsResponse,
  type UpdateActionRequest,
} from '../api/services';

export const useExecutorActionStore = defineStore('executor-action', () => {
  async function listActions(
    paging?: { page?: number; pageSize?: number },
    filters?: {
      name?: string;
      using?: string;
      enabled?: boolean;
    } | null,
  ): Promise<ListActionsResponse> {
    return await ActionService.list({
      page: paging?.page,
      pageSize: paging?.pageSize,
      name: filters?.name,
      using: filters?.using,
      enabled: filters?.enabled,
    });
  }

  async function getAction(id: string): Promise<{ action: Action }> {
    return await ActionService.get(id);
  }

  async function createAction(
    data: CreateActionRequest,
  ): Promise<{ action: Action }> {
    return await ActionService.create(data);
  }

  async function updateAction(
    id: string,
    data: UpdateActionRequest,
  ): Promise<{ action: Action }> {
    return await ActionService.update(id, data);
  }

  async function deleteAction(id: string): Promise<void> {
    return await ActionService.delete(id);
  }

  function $reset() {}

  return {
    $reset,
    listActions,
    getAction,
    createAction,
    updateAction,
    deleteAction,
  };
});
