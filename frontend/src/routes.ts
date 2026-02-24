import type { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    path: '/executor',
    name: 'Executor',
    component: () => import('shell/app-layout'),
    redirect: '/executor/scripts',
    meta: {
      order: 2050,
      icon: 'lucide:terminal',
      title: 'executor.menu.executor',
      keepAlive: true,
      authority: ['platform:admin', 'tenant:manager'],
    },
    children: [
      {
        path: 'scripts',
        name: 'ExecutorScripts',
        meta: {
          icon: 'lucide:file-code',
          title: 'executor.menu.scripts',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/scripts/index.vue'),
      },
      {
        path: 'executions',
        name: 'ExecutorExecutions',
        meta: {
          icon: 'lucide:play',
          title: 'executor.menu.executions',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/executions/index.vue'),
      },
    ],
  },
];

export default routes;
