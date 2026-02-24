/**
 * Thin LCM API wrapper for mTLS certificate search.
 * Used by the assignment drawer to search for client IDs.
 */

import { useAccessStore } from 'shell/vben/stores';

const LCM_BASE_URL = '/admin/v1/modules/lcm/v1';

export interface MtlsCertificate {
  serialNumber?: string;
  clientId?: string;
  commonName?: string;
  tenantId?: number;
  issuerName?: string;
  status?: string;
  certType?: string;
}

export interface ListMtlsCertificatesResponse {
  items?: MtlsCertificate[];
  total?: number;
}

export const MtlsCertificateService = {
  list: async (
    params?: {
      commonName?: string;
      pageSize?: number;
    },
  ): Promise<ListMtlsCertificatesResponse> => {
    const accessStore = useAccessStore();
    const token = accessStore.accessToken;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (token) headers.Authorization = `Bearer ${token}`;

    const query = new URLSearchParams();
    if (params?.commonName) query.set('commonName', params.commonName);
    if (params?.pageSize) query.set('pageSize', String(params.pageSize));
    const qs = query.toString();

    const response = await fetch(
      `${LCM_BASE_URL}/certificates${qs ? `?${qs}` : ''}`,
      { headers },
    );
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    return response.json();
  },
};
