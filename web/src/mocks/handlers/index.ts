import { patentHandlers } from './patents';
import { moleculeHandlers } from './molecules';
import { infringementHandlers } from './infringement';
import { portfolioHandlers } from './portfolio';
import { lifecycleHandlers } from './lifecycle';
import { partnerHandlers } from './partners';
import { dashboardHandlers } from './dashboard';

export const handlers = [
  ...patentHandlers,
  ...moleculeHandlers,
  ...infringementHandlers,
  ...portfolioHandlers,
  ...lifecycleHandlers,
  ...partnerHandlers,
  ...dashboardHandlers,
];
