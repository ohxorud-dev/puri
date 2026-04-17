import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

// Dev: same-origin via Vite proxy, Prod: direct API subdomain (same-site)
export const API_BASE_URL = import.meta.env.DEV ? "" : "https://api.puri.ac";

// Transport configuration — custom fetch to include credentials (cookies)
const transport = createConnectTransport({
  baseUrl: API_BASE_URL,
  fetch: (input, init) => fetch(input, { ...init, credentials: "include" }),
});

// Import services from unified _pb.ts files (v2)
import { UserService } from "../gen/user/v1/user_pb";
import { SubmissionService } from "../gen/submission/v1/submission_pb";
import { CommunityService } from "../gen/community/v1/community_pb";
import { ProposalService } from "../gen/proposal/v1/proposal_pb";

// Create clients
export const userClient = createClient(UserService, transport);
export const submissionClient = createClient(SubmissionService, transport);
export const communityClient = createClient(CommunityService, transport);
export const proposalClient = createClient(ProposalService, transport);
