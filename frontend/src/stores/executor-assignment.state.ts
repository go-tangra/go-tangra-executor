import { defineStore } from 'pinia';

import {
  AssignmentService,
  type ListAssignmentsResponse,
  type ListClientScriptsResponse,
  type ScriptAssignment,
} from '../api/services';

export const useExecutorAssignmentStore = defineStore(
  'executor-assignment',
  () => {
    async function assignScript(
      scriptId: string,
      clientId: string,
    ): Promise<{ assignment: ScriptAssignment }> {
      return await AssignmentService.assign(scriptId, clientId);
    }

    async function unassignScript(
      scriptId: string,
      clientId: string,
    ): Promise<void> {
      return await AssignmentService.unassign(scriptId, clientId);
    }

    async function listAssignments(
      scriptId: string,
    ): Promise<ListAssignmentsResponse> {
      return await AssignmentService.list(scriptId);
    }

    async function listClientScripts(
      clientId: string,
    ): Promise<ListClientScriptsResponse> {
      return await AssignmentService.listClientScripts(clientId);
    }

    function $reset() {}

    return {
      $reset,
      assignScript,
      unassignScript,
      listAssignments,
      listClientScripts,
    };
  },
);
