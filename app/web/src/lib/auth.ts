import { userClient } from "./api";
import type { User } from "../gen/user/v1/user_pb";

let cachedUser: User | null | undefined = undefined;
let userCheckPromise: Promise<User | null> | null = null;

export function hasSessionCookie(): boolean {
  if (typeof window === 'undefined') return false;
  return localStorage.getItem('is_logged_in') === '1';
}

export function clearSessionCookie(): void {
  if (typeof window !== 'undefined') {
    localStorage.removeItem('is_logged_in');
  }
}

export async function getUser(): Promise<User | null> {
  if (cachedUser !== undefined) {
    return cachedUser;
  }
  
  if (!userCheckPromise) {
    userCheckPromise = userClient.getProfile({})
      .then(resp => {
        cachedUser = resp.user ?? null;
        return cachedUser;
      })
      .catch(err => {
        cachedUser = null;
        clearSessionCookie();
        return null;
      });
  }
  
  return userCheckPromise;
}

export async function isLoggedIn(): Promise<boolean> {
  if (hasSessionCookie()) {
    // Kick off background profile fetch if not already done
    getUser().catch(() => {});
    return true;
  }
  return false;
}

export function clearAuthCache(): void {
  cachedUser = undefined;
  userCheckPromise = null;
  clearSessionCookie();
}
