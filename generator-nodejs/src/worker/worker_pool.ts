import { AsyncResource } from 'async_hooks';
import { EventEmitter } from 'events';
import * as path from 'path';
import { Worker } from 'worker_threads';

const kTaskInfo = Symbol('kTaskInfo');
const kWorkerFreedEvent = Symbol('kWorkerFreedEvent');

class WorkerPoolTaskInfo extends AsyncResource {

  callback : (err: Error, result: any) => any;

  constructor(callback: (err: Error, result: any) => any) {
    super('WorkerPoolTaskInfo');
    this.callback = callback;
  }

  done(err: Error, result:any) {
    this.runInAsyncScope(this.callback, null, err, result);
    this.emitDestroy();  // `TaskInfo`s are used only once.
  }
}

export default class WorkerPool extends EventEmitter {
  numThreads:number;
  workers: Worker[];
  freeWorkers: Worker[];
  constructor(numThreads: number) {
    super();
    this.numThreads = numThreads;
    this.workers = [];
    this.freeWorkers = [];

    for (let i = 0; i < numThreads; i++)
      this.addNewWorker();
  }

  addNewWorker() {
    const worker = new Worker(path.resolve(__dirname, 'sendTx.js'));
    worker.on('message', (result) => {
      // In case of success: Call the callback that was passed to `runTask`,
      // remove the `TaskInfo` associated with the Worker, and mark it as free
      // again.
      worker[kTaskInfo].done(null, result);
      worker[kTaskInfo] = null;
      this.freeWorkers.push(worker);
      this.emit(kWorkerFreedEvent);
    });
    worker.on('error', (err) => {
      // In case of an uncaught exception: Call the callback that was passed to
      // `runTask` with the error.
      if (worker[kTaskInfo])
        worker[kTaskInfo].done(err, null);
      else
        this.emit('error', err);
      // Remove the worker from the list and start a new Worker to replace the
      // current one.
      this.workers.splice(this.workers.indexOf(worker), 1);
      this.addNewWorker();
    });
    this.workers.push(worker);
    this.freeWorkers.push(worker);
    this.emit(kWorkerFreedEvent);
  }

  runTask<Task>(task: Task, callback:(err: any, result:any)=>any) {
    if (this.freeWorkers.length === 0) {
      // No free threads, wait until a worker thread becomes free.
      this.once(kWorkerFreedEvent, () => this.runTask(task, callback));
      return;
    }

    const worker = this.freeWorkers.pop();
    worker[kTaskInfo] = new WorkerPoolTaskInfo(callback);
    worker.postMessage(task);
  }

  close() {
    for (const worker of this.workers) worker.terminate();
  }
}
