/**
 * Executor Module Service Functions
 *
 * Typed service methods for the Executor API using dynamic module routing.
 * Base URL: /admin/v1/modules/executor/v1
 */

import { executorApi, type RequestOptions } from './client';

// ==================== Enums ====================

export type ScriptType =
  | 'SCRIPT_TYPE_BASH'
  | 'SCRIPT_TYPE_JAVASCRIPT'
  | 'SCRIPT_TYPE_LUA';

export type TriggerType = 'TRIGGER_TYPE_CLIENT_PULL' | 'TRIGGER_TYPE_UI_PUSH';

export type ExecutionStatus =
  | 'EXECUTION_STATUS_PENDING'
  | 'EXECUTION_STATUS_RUNNING'
  | 'EXECUTION_STATUS_COMPLETED'
  | 'EXECUTION_STATUS_FAILED'
  | 'EXECUTION_STATUS_REJECTED_HASH_MISMATCH'
  | 'EXECUTION_STATUS_REJECTED_NOT_APPROVED'
  | 'EXECUTION_STATUS_CLIENT_OFFLINE';

// ==================== Entity Types ====================

export interface Script {
  id: string;
  tenantId: number;
  name: string;
  description: string;
  scriptType: ScriptType;
  content: string;
  contentHash: string;
  version: number;
  enabled: boolean;
  createdBy?: number;
  updatedBy?: number;
  createTime: string;
  updateTime?: string;
}

export interface ScriptAssignment {
  id: string;
  tenantId: number;
  scriptId: string;
  clientId: string;
  createdBy?: number;
  createTime: string;
  script?: Script;
}

export interface ExecutionLog {
  id: string;
  tenantId: number;
  scriptId: string;
  scriptName: string;
  clientId: string;
  scriptHash: string;
  triggerType: TriggerType;
  status: ExecutionStatus;
  exitCode?: number;
  output?: string;
  errorOutput?: string;
  rejectionReason?: string;
  startedAt?: string;
  completedAt?: string;
  durationMs?: number;
  createdBy?: number;
  createTime: string;
}

// ==================== Request/Response Types ====================

export interface CreateScriptRequest {
  name: string;
  description?: string;
  scriptType: ScriptType;
  content: string;
  enabled?: boolean;
}

export interface UpdateScriptRequest {
  name?: string;
  description?: string;
  content?: string;
  enabled?: boolean;
  password?: string;
}

export interface ListScriptsResponse {
  scripts: Script[];
  total: number;
}

export interface ListAssignmentsResponse {
  assignments: ScriptAssignment[];
}

export interface ListClientScriptsResponse {
  assignments: ScriptAssignment[];
}

export interface ListExecutionsResponse {
  executions: ExecutionLog[];
  total: number;
}

export interface GetExecutionOutputResponse {
  output: string;
  errorOutput: string;
  exitCode?: number;
}

// ==================== Script Service ====================

export const ScriptService = {
  create: (data: CreateScriptRequest, options?: RequestOptions) =>
    executorApi.post<{ script: Script }>('/scripts', data, options),

  get: (id: string, options?: RequestOptions) =>
    executorApi.get<{ script: Script }>(`/scripts/${id}`, options),

  list: (
    params?: {
      page?: number;
      pageSize?: number;
      scriptType?: string;
      name?: string;
      enabled?: boolean;
    },
    options?: RequestOptions,
  ) => {
    const query = new URLSearchParams();
    if (params?.page) query.set('page', String(params.page));
    if (params?.pageSize) query.set('pageSize', String(params.pageSize));
    if (params?.scriptType) query.set('scriptType', params.scriptType);
    if (params?.name) query.set('name', params.name);
    if (params?.enabled !== undefined)
      query.set('enabled', String(params.enabled));
    const qs = query.toString();
    return executorApi.get<ListScriptsResponse>(
      `/scripts${qs ? `?${qs}` : ''}`,
      options,
    );
  },

  update: (
    id: string,
    data: UpdateScriptRequest,
    options?: RequestOptions,
  ) => executorApi.put<{ script: Script }>(`/scripts/${id}`, data, options),

  delete: (id: string, options?: RequestOptions) =>
    executorApi.delete<void>(`/scripts/${id}`, options),
};

// ==================== Assignment Service ====================

export const AssignmentService = {
  assign: (
    scriptId: string,
    clientId: string,
    options?: RequestOptions,
  ) =>
    executorApi.post<{ assignment: ScriptAssignment }>(
      `/scripts/${scriptId}/assignments`,
      { clientId },
      options,
    ),

  unassign: (
    scriptId: string,
    clientId: string,
    options?: RequestOptions,
  ) =>
    executorApi.delete<void>(
      `/scripts/${scriptId}/assignments/${clientId}`,
      options,
    ),

  list: (scriptId: string, options?: RequestOptions) =>
    executorApi.get<ListAssignmentsResponse>(
      `/scripts/${scriptId}/assignments`,
      options,
    ),

  listClientScripts: (clientId: string, options?: RequestOptions) =>
    executorApi.get<ListClientScriptsResponse>(
      `/clients/${clientId}/scripts`,
      options,
    ),
};

// ==================== Client Update Types ====================

export interface TriggerClientUpdateResponse {
  commandId: string;
  clientOnline: boolean;
}

// ==================== Client Update Service ====================

export const ClientUpdateService = {
  trigger: (
    clientId: string,
    targetVersion?: string,
    options?: RequestOptions,
  ) =>
    executorApi.post<TriggerClientUpdateResponse>(
      `/clients/${clientId}/update`,
      { targetVersion: targetVersion || '' },
      options,
    ),
};

// ==================== Execution Service ====================

export const ExecutionService = {
  trigger: (
    scriptId: string,
    clientId: string,
    options?: RequestOptions,
  ) =>
    executorApi.post<{ execution: ExecutionLog }>(
      `/scripts/${scriptId}/execute`,
      { clientId },
      options,
    ),

  get: (id: string, options?: RequestOptions) =>
    executorApi.get<{ execution: ExecutionLog }>(`/executions/${id}`, options),

  list: (
    params?: {
      page?: number;
      pageSize?: number;
      scriptId?: string;
      clientId?: string;
      status?: string;
    },
    options?: RequestOptions,
  ) => {
    const query = new URLSearchParams();
    if (params?.page) query.set('page', String(params.page));
    if (params?.pageSize) query.set('pageSize', String(params.pageSize));
    if (params?.scriptId) query.set('scriptId', params.scriptId);
    if (params?.clientId) query.set('clientId', params.clientId);
    if (params?.status) query.set('status', params.status);
    const qs = query.toString();
    return executorApi.get<ListExecutionsResponse>(
      `/executions${qs ? `?${qs}` : ''}`,
      options,
    );
  },

  getOutput: (id: string, options?: RequestOptions) =>
    executorApi.get<GetExecutionOutputResponse>(
      `/executions/${id}/output`,
      options,
    ),
};

// ==================== Action Repository Types ====================

export interface ActionFile {
  path: string;
  content: string;
}

export interface Action {
  id: string;
  tenantId: number;
  name: string;
  description: string;
  using: string;
  manifest: string;
  contentHash: string;
  version: number;
  enabled: boolean;
  files: ActionFile[];
  createdAt?: string;
  updatedAt?: string;
}

export interface CreateActionRequest {
  name: string;
  description?: string;
  manifest: string;
  files?: ActionFile[];
  enabled?: boolean;
}

export interface UpdateActionRequest {
  description?: string;
  manifest?: string;
  files?: ActionFile[];
  enabled?: boolean;
  password?: string;
}

export interface ListActionsResponse {
  actions: Action[];
  total: number;
}

// ==================== Action Service ====================

export const ActionService = {
  create: (data: CreateActionRequest, options?: RequestOptions) =>
    executorApi.post<{ action: Action }>('/actions', data, options),

  get: (id: string, options?: RequestOptions) =>
    executorApi.get<{ action: Action }>(`/actions/${id}`, options),

  list: (
    params?: {
      page?: number;
      pageSize?: number;
      name?: string;
      using?: string;
      enabled?: boolean;
    },
    options?: RequestOptions,
  ) => {
    const query = new URLSearchParams();
    if (params?.page) query.set('page', String(params.page));
    if (params?.pageSize) query.set('pageSize', String(params.pageSize));
    if (params?.name) query.set('name', params.name);
    if (params?.using) query.set('using', params.using);
    if (params?.enabled !== undefined)
      query.set('enabled', String(params.enabled));
    const qs = query.toString();
    return executorApi.get<ListActionsResponse>(
      `/actions${qs ? `?${qs}` : ''}`,
      options,
    );
  },

  update: (id: string, data: UpdateActionRequest, options?: RequestOptions) =>
    executorApi.put<{ action: Action }>(`/actions/${id}`, data, options),

  delete: (id: string, options?: RequestOptions) =>
    executorApi.delete<void>(`/actions/${id}`, options),
};

// ==================== Workflow Repository Types ====================

export interface Workflow {
  id: string;
  tenantId: number;
  name: string;
  description: string;
  content: string;
  contentHash: string;
  version: number;
  enabled: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export interface CreateWorkflowRequest {
  name: string;
  description?: string;
  content: string;
  enabled?: boolean;
}

export interface UpdateWorkflowRequest {
  name?: string;
  description?: string;
  content?: string;
  enabled?: boolean;
}

export interface ListWorkflowsResponse {
  workflows: Workflow[];
  total: number;
}

// ==================== Workflow Service ====================

export const WorkflowService = {
  create: (data: CreateWorkflowRequest, options?: RequestOptions) =>
    executorApi.post<{ workflow: Workflow }>('/workflows', data, options),

  get: (id: string, options?: RequestOptions) =>
    executorApi.get<{ workflow: Workflow }>(`/workflows/${id}`, options),

  list: (
    params?: { page?: number; pageSize?: number; name?: string; enabled?: boolean },
    options?: RequestOptions,
  ) => {
    const query = new URLSearchParams();
    if (params?.page) query.set('page', String(params.page));
    if (params?.pageSize) query.set('pageSize', String(params.pageSize));
    if (params?.name) query.set('name', params.name);
    if (params?.enabled !== undefined)
      query.set('enabled', String(params.enabled));
    const qs = query.toString();
    return executorApi.get<ListWorkflowsResponse>(
      `/workflows${qs ? `?${qs}` : ''}`,
      options,
    );
  },

  update: (id: string, data: UpdateWorkflowRequest, options?: RequestOptions) =>
    executorApi.put<{ workflow: Workflow }>(`/workflows/${id}`, data, options),

  delete: (id: string, options?: RequestOptions) =>
    executorApi.delete<void>(`/workflows/${id}`, options),
};

// ==================== Connected Clients ====================

export interface ConnectedClient {
  clientId: string;
  clientVersion: string;
  connectedAt?: string;
  actionsEnabled?: boolean;
  securityHardened?: boolean;
}

export interface ListConnectedClientsResponse {
  clients: ConnectedClient[];
}

export const ConnectedClientsService = {
  list: (options?: RequestOptions) =>
    executorApi.get<ListConnectedClientsResponse>('/clients/connected', options),
};

// ==================== Run Workflow ====================

export interface TriggerWorkflowRequest {
  name?: string;
  workflow: string;
  inputs?: Record<string, string>;
}

export const WorkflowRunService = {
  run: (
    clientId: string,
    data: TriggerWorkflowRequest,
    options?: RequestOptions,
  ) =>
    executorApi.post<{ execution: ExecutionLog }>(
      `/clients/${clientId}/run-workflow`,
      data,
      options,
    ),
};

// ==================== Inventory agent settings ====================

/**
 * Platform-wide settings handed to go-tangra-client so it can install the
 * go-tangra v4 inventory agent and enroll it with an auto-enrollment key.
 * The key secret is write-only: it is never returned.
 */
export interface InventoryAgentSettings {
  enabled: boolean;
  ingestEndpoint: string;
  keyId: string;
  keySecretConfigured: boolean;
  caPem: string;
  serverName: string;
  agentVersion: string;
  updateTime?: string;
  updatedBy?: number;
}

export interface UpdateInventoryAgentSettingsRequest {
  enabled: boolean;
  ingestEndpoint: string;
  keyId: string;
  keySecret?: string;
  clearKeySecret?: boolean;
  caPem: string;
  serverName: string;
  agentVersion: string;
}

export const InventoryAgentSettingsService = {
  get: (options?: RequestOptions) =>
    executorApi.get<InventoryAgentSettings>('/settings/inventory-agent', options),
  update: (body: UpdateInventoryAgentSettingsRequest, options?: RequestOptions) =>
    executorApi.put<InventoryAgentSettings>('/settings/inventory-agent', body, options),
};
