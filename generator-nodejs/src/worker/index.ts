import WorkerPool from "./worker_pool";

const pool = new WorkerPool(os.cpus().length);

const worker = new Worker('./sendTx.js')
