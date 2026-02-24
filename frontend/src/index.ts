import type { TangraModule } from './sdk';
import routes from './routes';
import { useExecutorScriptStore } from './stores/executor-script.state';
import { useExecutorAssignmentStore } from './stores/executor-assignment.state';
import { useExecutorExecutionStore } from './stores/executor-execution.state';
import enUS from './locales/en-US.json';

const executorModule: TangraModule = {
  id: 'executor',
  version: '1.0.0',
  routes,
  stores: {
    'executor-script': useExecutorScriptStore,
    'executor-assignment': useExecutorAssignmentStore,
    'executor-execution': useExecutorExecutionStore,
  },
  locales: {
    'en-US': enUS,
  },
};

export default executorModule;
