import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { SubmissionService } from "./gen/submission/v1/submission_pb";
import { UserService } from "./gen/user/v1/user_pb";

const API_BASE_URL = import.meta.env.DEV ? "" : "https://api.puri.ac";

const transport = createConnectTransport({
  baseUrl: API_BASE_URL,
  fetch: (input, init) => fetch(input, { ...init, credentials: "include" }),
});

export const userClient = createClient(UserService, transport);
export const submissionClient = createClient(SubmissionService, transport);
