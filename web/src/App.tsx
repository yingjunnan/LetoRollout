import { useEffect } from "react";
import { useSession } from "./state/store";
import { TokenGate } from "./components/auth/TokenGate";
import { Shell } from "./components/layout/Shell";
import { Toasts } from "./components/ui/Toast";

export default function App() {
  const token = useSession((s) => s.token);
  const init = useSession((s) => s.init);

  useEffect(() => {
    init();
  }, [init]);

  // If we have a token, show the shell. The first API call re-verifies it
  // implicitly; a stale token returns 401 and the hook logs us out, dropping
  // back to the gate.
  return (
    <>
      {token ? <Shell /> : <TokenGate />}
      <Toasts />
    </>
  );
}
