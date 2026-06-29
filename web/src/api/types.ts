// Types mirroring the Go rollout/auth domain. Kept hand-aligned (no codegen)
// so the frontend stays dependency-light.

export interface ContainerInfo {
  name: string;
  image: string;
}

export interface DeploymentSummary {
  name: string;
  namespace: string;
  replicas: number;
  readyReplicas: number;
  containers: ContainerInfo[];
}

export interface DeploymentDetail extends DeploymentSummary {
  selector: string;
}

export interface RolloutResult {
  namespace: string;
  deployment: string;
  container: string;
  oldImage: string;
  newImage: string;
  generation: number;
  dryRun: boolean;
  rolloutComplete: boolean;
}

export interface ImageUpdateRequest {
  container: string;
  image: string;
  dryRun?: boolean;
  wait?: boolean;
  timeoutSeconds?: number;
}

export interface TokenScope {
  namespace: string;
  deployment: string;
}

export interface VerifyResponse {
  isAdmin: boolean;
  scopes: TokenScope[];
}

export interface TokenRecord {
  id: string;
  token?: string; // only present right after create
  label: string;
  scopes: TokenScope[];
  expiresAt?: string | null;
  createdAt: string;
}
