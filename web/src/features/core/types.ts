export type CoreProcessState = "not_installed" | "stopped" | "starting" | "running" | "stopping" | "start_failed" | "exited";

export interface CoreOperation {
  active: boolean;
  kind?: string;
  stage?: string;
  startedAt?: string;
  error?: string;
}

export interface CoreStatus {
  installedVersion: string;
  installedDigest: string;
  state: CoreProcessState;
  pid: number;
  currentProfileId: string;
  appliedRevision: number | null;
  expectedRunning: boolean;
  configDirectoryDigest: string;
  startedAt: string | null;
  lastError: string;
  operation: CoreOperation;
}

export interface LatestRelease {
  version: string;
  arch: string;
  /** Present in fakes only; production derives this from installedVersion. */
  updateAvailable?: boolean;
}

export interface CoreUpdateStatus {
  operationId: string;
  targetVersion?: string;
  stage: string;
  startedAt: string;
  finishedAt?: string;
  message?: string;
  downloadedBytes?: number;
  totalBytes?: number;
  bytesPerSecond?: number;
}
