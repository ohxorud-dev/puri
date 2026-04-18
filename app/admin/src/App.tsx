import { useCallback, useEffect, useRef, useState } from "react";
import { submissionClient, userClient } from "./api";
import { Language, SubmissionStatus } from "./gen/common/v1/types_pb";
import type { Submission } from "./gen/submission/v1/submission_pb";
import type { User } from "./gen/user/v1/user_pb";

type AuthState =
  | { status: "loading" }
  | { status: "signed-out" }
  | { status: "denied"; user: User }
  | { status: "ready"; user: User };

type Tab = "users" | "submissions";

export function App() {
  const [auth, setAuth] = useState<AuthState>({ status: "loading" });

  const refresh = useCallback(async () => {
    try {
      const resp = await userClient.getProfile({});
      const u = resp.user;
      if (!u) {
        setAuth({ status: "signed-out" });
        return;
      }
      if (u.role !== "admin") {
        setAuth({ status: "denied", user: u });
        return;
      }
      setAuth({ status: "ready", user: u });
    } catch {
      setAuth({ status: "signed-out" });
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  if (auth.status === "loading") {
    return <div className="empty">불러오는 중...</div>;
  }

  if (auth.status === "signed-out") {
    return <LoginCard onSuccess={refresh} />;
  }

  if (auth.status === "denied") {
    return (
      <div className="empty">
        <p>관리자 권한이 없습니다. ({auth.user.username})</p>
        <button
          className="btn"
          onClick={async () => {
            try {
              await userClient.logout({});
            } catch {}
            refresh();
          }}
        >
          로그아웃
        </button>
      </div>
    );
  }

  return <AdminShell user={auth.user} onLogout={refresh} />;
}

function LoginCard({ onSuccess }: { onSuccess: () => void }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErr(null);
    setLoading(true);
    try {
      await userClient.login({ username, password });
      onSuccess();
    } catch (e: any) {
      setErr(e?.message ?? "로그인 실패");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-card">
      <h2>관리자 로그인</h2>
      <form onSubmit={submit}>
        <div className="field">
          <label htmlFor="u">아이디</label>
          <input
            id="u"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoComplete="username"
            required
          />
        </div>
        <div className="field">
          <label htmlFor="p">비밀번호</label>
          <input
            id="p"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </div>
        {err && <div className="error">{err}</div>}
        <button className="btn btn-primary" type="submit" disabled={loading}>
          {loading ? "로그인 중..." : "로그인"}
        </button>
      </form>
    </div>
  );
}

function AdminShell({ user, onLogout }: { user: User; onLogout: () => void }) {
  const [tab, setTab] = useState<Tab>("users");

  const handleLogout = async () => {
    try {
      await userClient.logout({});
    } catch {}
    onLogout();
  };

  return (
    <div className="app">
      <header className="app-header">
        <h1>Puri 관리자</h1>
        <div className="user-badge">
          <span>{user.username} (admin)</span>
          <button className="btn" onClick={handleLogout}>
            로그아웃
          </button>
        </div>
      </header>

      <nav className="tabs">
        <button
          className={`tab ${tab === "users" ? "active" : ""}`}
          onClick={() => setTab("users")}
        >
          유저
        </button>
        <button
          className={`tab ${tab === "submissions" ? "active" : ""}`}
          onClick={() => setTab("submissions")}
        >
          제출
        </button>
      </nav>

      {tab === "users" && <UsersPanel self={user} />}
      {tab === "submissions" && <SubmissionsPanel />}
    </div>
  );
}

const PAGE_SIZE = 50;

type MenuItem = {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  variant?: "default" | "danger" | "success";
};

function ActionMenu({ items }: { items: MenuItem[] }) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ top: number; right: number } | null>(null);
  const btnRef = useRef<HTMLButtonElement>(null);
  const ddRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (btnRef.current?.contains(e.target as Node) || ddRef.current?.contains(e.target as Node))
        return;
      setOpen(false);
    };
    const esc = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    const reposition = () => setOpen(false);
    document.addEventListener("mousedown", handler);
    document.addEventListener("keydown", esc);
    window.addEventListener("scroll", reposition, true);
    window.addEventListener("resize", reposition);
    return () => {
      document.removeEventListener("mousedown", handler);
      document.removeEventListener("keydown", esc);
      window.removeEventListener("scroll", reposition, true);
      window.removeEventListener("resize", reposition);
    };
  }, [open]);

  const toggle = () => {
    if (open) {
      setOpen(false);
      return;
    }
    const rect = btnRef.current?.getBoundingClientRect();
    if (rect) {
      setPos({
        top: rect.bottom + 4,
        right: window.innerWidth - rect.right,
      });
    }
    setOpen(true);
  };

  return (
    <>
      <button
        ref={btnRef}
        className="icon-btn"
        aria-label="액션"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={toggle}
      >
        ⋮
      </button>
      {open && pos && (
        <div
          ref={ddRef}
          className="action-menu-dropdown"
          role="menu"
          style={{ top: pos.top, right: pos.right }}
        >
          {items.map((item, i) => (
            <button
              key={i}
              role="menuitem"
              className={`action-menu-item action-menu-item-${item.variant ?? "default"}`}
              disabled={item.disabled}
              onClick={() => {
                setOpen(false);
                item.onClick();
              }}
            >
              {item.label}
            </button>
          ))}
        </div>
      )}
    </>
  );
}

function UsersPanel({ self }: { self: User }) {
  const [users, setUsers] = useState<User[]>([]);
  const [pageStack, setPageStack] = useState<string[]>([]);
  const [nextToken, setNextToken] = useState("");
  const [filter, setFilter] = useState("");
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [actingId, setActingId] = useState<bigint | null>(null);

  const loadPage = useCallback(async (pageToken: string) => {
    setLoading(true);
    setErr(null);
    try {
      const resp = await userClient.adminListUsers({
        pageSize: PAGE_SIZE,
        pageToken,
      });
      setUsers(resp.users ?? []);
      setNextToken(resp.nextPageToken ?? "");
    } catch (e: any) {
      setErr(e?.message ?? "목록 로드 실패");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadPage("");
  }, [loadPage]);

  const currentToken = pageStack[pageStack.length - 1] ?? "";
  const reload = () => loadPage(currentToken);

  const handleBan = async (u: User) => {
    const reason = prompt(`"${u.username}" 유저 차단 사유 (비워두면 "부정 사용"):`, "");
    if (reason == null) return;
    setActingId(u.id);
    try {
      await userClient.adminBanUser({ userId: u.id, reason: reason.trim() });
      await loadPage(currentToken);
    } catch (e: any) {
      alert(`실패: ${e?.message ?? String(e)}`);
    } finally {
      setActingId(null);
    }
  };

  const handleUnban = async (u: User) => {
    setActingId(u.id);
    try {
      await userClient.adminUnbanUser({ userId: u.id });
      await loadPage(currentToken);
    } catch (e: any) {
      alert(`실패: ${e?.message ?? String(e)}`);
    } finally {
      setActingId(null);
    }
  };

  const handleUpdateReason = async (u: User) => {
    const current = u.activeBan?.reason ?? "";
    const reason = prompt(`"${u.username}" 차단 사유 변경 (비워두면 기본값):`, current);
    if (reason == null) return;
    setActingId(u.id);
    try {
      await userClient.adminUpdateBanReason({
        userId: u.id,
        reason: reason.trim(),
      });
      await loadPage(currentToken);
    } catch (e: any) {
      alert(`실패: ${e?.message ?? String(e)}`);
    } finally {
      setActingId(null);
    }
  };

  const handleSetRole = async (u: User, role: "user" | "admin") => {
    const label = role === "admin" ? "관리자" : "일반 유저";
    if (!confirm(`"${u.username}" 을(를) ${label}(으)로 변경합니다.`)) return;
    setActingId(u.id);
    try {
      await userClient.adminSetUserRole({ userId: u.id, role });
      await loadPage(currentToken);
    } catch (e: any) {
      alert(`실패: ${e?.message ?? String(e)}`);
    } finally {
      setActingId(null);
    }
  };

  const filtered = filter
    ? users.filter((u) => u.username.toLowerCase().includes(filter.toLowerCase()))
    : users;

  return (
    <>
      <div className="toolbar">
        <input
          type="text"
          placeholder="유저명 필터..."
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        <button className="btn" onClick={reload} disabled={loading}>
          {loading ? "..." : "새로고침"}
        </button>
      </div>

      {err && <div className="error">{err}</div>}

      <div className="table-wrap">
        {filtered.length === 0 ? (
          <div className="empty">{loading ? "불러오는 중..." : "유저 없음"}</div>
        ) : (
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>유저명</th>
                <th>이메일</th>
                <th>역할</th>
                <th>상태</th>
                <th style={{ width: "1%" }}>액션</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((u) => (
                <tr key={u.id.toString()}>
                  <td>{u.id.toString()}</td>
                  <td>{u.username}</td>
                  <td>{u.email}</td>
                  <td>
                    <span className={u.role === "admin" ? "badge badge-admin" : "badge badge-user"}>
                      {u.role || "user"}
                    </span>
                  </td>
                  <td>
                    {u.isBanned ? (
                      <div>
                        <span className="status-banned">차단됨</span>
                        {u.activeBan?.reason && (
                          <div className="ban-reason" title={u.activeBan.reason}>
                            {u.activeBan.reason}
                          </div>
                        )}
                      </div>
                    ) : (
                      <span className="status-active">정상</span>
                    )}
                  </td>
                  <td>
                    {u.id === self.id ? (
                      <span style={{ color: "var(--muted)" }}>본인</span>
                    ) : (
                      <ActionMenu
                        items={[
                          ...(u.isBanned
                            ? [
                                {
                                  label: "차단 사유 변경",
                                  disabled: actingId === u.id,
                                  onClick: () => handleUpdateReason(u),
                                },
                                {
                                  label: "차단 해제",
                                  variant: "success" as const,
                                  disabled: actingId === u.id,
                                  onClick: () => handleUnban(u),
                                },
                              ]
                            : [
                                {
                                  label: "차단",
                                  variant: "danger" as const,
                                  disabled: actingId === u.id,
                                  onClick: () => handleBan(u),
                                },
                              ]),
                          {
                            label: u.role === "admin" ? "일반 유저로 변경" : "관리자로 변경",
                            disabled: actingId === u.id,
                            onClick: () => handleSetRole(u, u.role === "admin" ? "user" : "admin"),
                          },
                        ]}
                      />
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <Pagination
        pageStack={pageStack}
        setPageStack={setPageStack}
        nextToken={nextToken}
        loading={loading}
        loadPage={loadPage}
      />
    </>
  );
}

function SubmissionsPanel() {
  const [submissions, setSubmissions] = useState<Submission[]>([]);
  const [pageStack, setPageStack] = useState<string[]>([]);
  const [nextToken, setNextToken] = useState("");
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const [userIdFilter, setUserIdFilter] = useState("");
  const [problemIdFilter, setProblemIdFilter] = useState("");
  const [selected, setSelected] = useState<Submission | null>(null);

  const loadPage = useCallback(
    async (pageToken: string) => {
      setLoading(true);
      setErr(null);
      try {
        const req: any = { pageSize: PAGE_SIZE, pageToken };
        if (userIdFilter.trim()) {
          try {
            req.userId = BigInt(userIdFilter.trim());
          } catch {}
        }
        if (problemIdFilter.trim()) {
          const n = parseInt(problemIdFilter.trim(), 10);
          if (!isNaN(n)) req.problemId = n;
        }
        const resp = await submissionClient.listSubmissions(req);
        setSubmissions(resp.submissions ?? []);
        setNextToken(resp.nextPageToken ?? "");
      } catch (e: any) {
        setErr(e?.message ?? "목록 로드 실패");
      } finally {
        setLoading(false);
      }
    },
    [userIdFilter, problemIdFilter],
  );

  useEffect(() => {
    loadPage("");
    setPageStack([]);
  }, [loadPage]);

  const currentToken = pageStack[pageStack.length - 1] ?? "";
  const reload = () => loadPage(currentToken);

  const handleDelete = async (s: Submission) => {
    if (!confirm(`제출 #${s.id.toString()} 를 삭제하시겠습니까?`)) return;
    try {
      await submissionClient.deleteSubmission({ submissionId: s.id });
      await loadPage(currentToken);
    } catch (e: any) {
      alert(`삭제 실패: ${e?.message ?? String(e)}`);
    }
  };

  return (
    <>
      <div className="toolbar">
        <input
          type="text"
          placeholder="유저 ID 필터"
          value={userIdFilter}
          onChange={(e) => setUserIdFilter(e.target.value)}
          style={{ maxWidth: 160 }}
        />
        <input
          type="text"
          placeholder="문제 ID 필터"
          value={problemIdFilter}
          onChange={(e) => setProblemIdFilter(e.target.value)}
          style={{ maxWidth: 160 }}
        />
        <button className="btn" onClick={reload} disabled={loading}>
          {loading ? "..." : "새로고침"}
        </button>
      </div>

      {err && <div className="error">{err}</div>}

      <div className="table-wrap">
        {submissions.length === 0 ? (
          <div className="empty">{loading ? "불러오는 중..." : "제출 없음"}</div>
        ) : (
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>유저</th>
                <th>문제</th>
                <th>언어</th>
                <th>상태</th>
                <th>시간</th>
                <th>메모리</th>
                <th style={{ width: "1%" }}>액션</th>
              </tr>
            </thead>
            <tbody>
              {submissions.map((s) => (
                <tr key={s.id.toString()}>
                  <td>{s.id.toString()}</td>
                  <td>{s.username || s.userId.toString()}</td>
                  <td>
                    {s.problemId}
                    {s.problemTitle ? ` — ${s.problemTitle}` : ""}
                  </td>
                  <td>{languageLabel(s.language)}</td>
                  <td>
                    <span className={statusClass(s.status)}>{statusLabel(s.status)}</span>
                  </td>
                  <td>{s.executionTimeMs > 0 ? `${s.executionTimeMs}ms` : "-"}</td>
                  <td>{s.memoryUsageKb > 0 ? `${s.memoryUsageKb}KB` : "-"}</td>
                  <td>
                    <ActionMenu
                      items={[
                        {
                          label: "코드 보기",
                          onClick: () => setSelected(s),
                        },
                        {
                          label: "제출 삭제",
                          variant: "danger",
                          onClick: () => handleDelete(s),
                        },
                      ]}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <Pagination
        pageStack={pageStack}
        setPageStack={setPageStack}
        nextToken={nextToken}
        loading={loading}
        loadPage={loadPage}
      />

      {selected && <SubmissionModal submission={selected} onClose={() => setSelected(null)} />}
    </>
  );
}

function SubmissionModal({ submission, onClose }: { submission: Submission; onClose: () => void }) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [onClose]);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(submission.sourceCode);
    } catch {}
  };

  return (
    <div
      className="modal-backdrop"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="modal">
        <header className="modal-header">
          <div>
            <div className="modal-title">
              제출 #{submission.id.toString()} —{" "}
              {submission.username || submission.userId.toString()}
            </div>
            <div className="modal-sub">
              문제 {submission.problemId}
              {submission.problemTitle ? ` · ${submission.problemTitle}` : ""}
              {" · "}
              {languageLabel(submission.language)}
              {" · "}
              <span className={statusClass(submission.status)}>
                {statusLabel(submission.status)}
              </span>
            </div>
          </div>
          <div className="modal-actions">
            <button className="btn" onClick={copy}>
              복사
            </button>
            <button className="btn" onClick={onClose}>
              닫기
            </button>
          </div>
        </header>
        <pre className="code-block">{submission.sourceCode}</pre>
      </div>
    </div>
  );
}

function Pagination({
  pageStack,
  setPageStack,
  nextToken,
  loading,
  loadPage,
}: {
  pageStack: string[];
  setPageStack: (v: string[]) => void;
  nextToken: string;
  loading: boolean;
  loadPage: (token: string) => void;
}) {
  return (
    <div className="pagination">
      <button
        className="btn"
        disabled={pageStack.length === 0 || loading}
        onClick={() => {
          const next = pageStack.slice(0, -1);
          setPageStack(next);
          loadPage(next[next.length - 1] ?? "");
        }}
      >
        이전
      </button>
      <span className="page-indicator">{pageStack.length + 1}</span>
      <button
        className="btn"
        disabled={!nextToken || loading}
        onClick={() => {
          const next = [...pageStack, nextToken];
          setPageStack(next);
          loadPage(nextToken);
        }}
      >
        다음
      </button>
    </div>
  );
}

function languageLabel(l: Language): string {
  switch (l) {
    case Language.CPP:
      return "C++";
    case Language.PYTHON:
      return "Python";
    case Language.JAVA:
      return "Java";
    case Language.GO:
      return "Go";
    case Language.JAVASCRIPT:
      return "JavaScript";
    case Language.RUST:
      return "Rust";
    default:
      return "?";
  }
}

function statusLabel(s: SubmissionStatus): string {
  switch (s) {
    case SubmissionStatus.PENDING:
      return "대기";
    case SubmissionStatus.JUDGING:
      return "채점중";
    case SubmissionStatus.ACCEPTED:
      return "맞았습니다";
    case SubmissionStatus.WRONG_ANSWER:
      return "틀렸습니다";
    case SubmissionStatus.TIME_LIMIT_EXCEEDED:
      return "시간 초과";
    case SubmissionStatus.RUNTIME_ERROR:
      return "런타임 에러";
    case SubmissionStatus.MEMORY_LIMIT_EXCEEDED:
      return "메모리 초과";
    case SubmissionStatus.COMPILATION_ERROR:
      return "컴파일 에러";
    default:
      return "?";
  }
}

function statusClass(s: SubmissionStatus): string {
  if (s === SubmissionStatus.ACCEPTED) return "status-active";
  if (s === SubmissionStatus.PENDING || s === SubmissionStatus.JUDGING) return "status-muted";
  return "status-banned";
}
