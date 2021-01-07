import { Worker, parentPort, workerData } from 'worker_threads';

const worker = new Worker('./sendTx.js')
