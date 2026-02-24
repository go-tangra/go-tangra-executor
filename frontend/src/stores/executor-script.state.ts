import { defineStore } from 'pinia';

import {
  ScriptService,
  type CreateScriptRequest,
  type ListScriptsResponse,
  type Script,
  type UpdateScriptRequest,
} from '../api/services';

export const useExecutorScriptStore = defineStore('executor-script', () => {
  async function listScripts(
    paging?: { page?: number; pageSize?: number },
    filters?: {
      scriptType?: string;
      name?: string;
      enabled?: boolean;
    } | null,
  ): Promise<ListScriptsResponse> {
    return await ScriptService.list({
      page: paging?.page,
      pageSize: paging?.pageSize,
      scriptType: filters?.scriptType,
      name: filters?.name,
      enabled: filters?.enabled,
    });
  }

  async function getScript(id: string): Promise<{ script: Script }> {
    return await ScriptService.get(id);
  }

  async function createScript(
    data: CreateScriptRequest,
  ): Promise<{ script: Script }> {
    return await ScriptService.create(data);
  }

  async function updateScript(
    id: string,
    data: UpdateScriptRequest,
  ): Promise<{ script: Script }> {
    return await ScriptService.update(id, data);
  }

  async function deleteScript(id: string): Promise<void> {
    return await ScriptService.delete(id);
  }

  function $reset() {}

  return {
    $reset,
    listScripts,
    getScript,
    createScript,
    updateScript,
    deleteScript,
  };
});
