import { api } from './adapter';
import { LifecycleEvent, Jurisdiction } from '../types/domain';
import { ApiResponse } from '../types/api';

export const lifecycleService = {
  async getEvents(jurisdiction?: Jurisdiction, status?: string): Promise<ApiResponse<LifecycleEvent[]>> {
    const params: Record<string, any> = {};
    if (jurisdiction) params.jurisdiction = jurisdiction;
    if (status) params.status = status;

    return api.get<ApiResponse<LifecycleEvent[]>>('/lifecycle/events', params);
  }
};
