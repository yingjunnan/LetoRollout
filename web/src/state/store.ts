import { create } from "zustand";
import type { TokenScope, VerifyResponse } from "../api/types";

const TOKEN_KEY = "letorollout.token";

interface SessionState {
  token: string | null;
  isAdmin: boolean;
  scopes: TokenScope[];
  // load a persisted token on boot
  init: () => void;
  setSession: (token: string, v: VerifyResponse) => void;
  logout: () => void;
}

export const useSession = create<SessionState>((set) => ({
  token: null,
  isAdmin: false,
  scopes: [],
  init: () => {
    const t = localStorage.getItem(TOKEN_KEY);
    if (t) set({ token: t });
  },
  setSession: (token, v) => {
    localStorage.setItem(TOKEN_KEY, token);
    set({ token, isAdmin: v.isAdmin, scopes: v.scopes });
  },
  logout: () => {
    localStorage.removeItem(TOKEN_KEY);
    set({ token: null, isAdmin: false, scopes: [] });
  },
}));

// ---- toast store (global notifications) ----
export interface Toast {
  id: number;
  kind: "info" | "error" | "success";
  message: string;
}

interface ToastState {
  toasts: Toast[];
  push: (kind: Toast["kind"], message: string) => void;
  dismiss: (id: number) => void;
}

let toastSeq = 0;

export const useToasts = create<ToastState>((set) => ({
  toasts: [],
  push: (kind, message) => {
    const id = ++toastSeq;
    set((s) => ({ toasts: [...s.toasts, { id, kind, message }] }));
    setTimeout(() => {
      set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) }));
    }, 4000);
  },
  dismiss: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
}));
