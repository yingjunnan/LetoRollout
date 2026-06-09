const stateKey = "letorollout-console-state-v1";
const recentTargetsKey = "letorollout-console-targets-v1";
const recentResultsKey = "letorollout-console-results-v1";

const state = loadState();
const els = {};

document.addEventListener("DOMContentLoaded", () => {
  cacheElements();
  wireEvents();
  hydrateForms();
  render();
  pingApi();
});

function cacheElements() {
  els.connection = document.getElementById("connection-state");
  els.recentTargets = document.getElementById("recent-targets");
  els.recentResults = document.getElementById("recent-results");
  els.resultBody = document.getElementById("result-body");
  els.resultPanel = document.getElementById("result-panel");
  els.envList = document.getElementById("env-list");
  els.createForm = document.getElementById("create-form");
  els.updateForm = document.getElementById("update-form");
  els.resetCreate = document.getElementById("reset-create");
  els.resetUpdate = document.getElementById("reset-update");
  els.addEnvRow = document.getElementById("add-env-row");
  els.tabs = Array.from(document.querySelectorAll("[data-tab]"));
  els.panes = Array.from(document.querySelectorAll("[data-pane]"));
}

function wireEvents() {
  els.tabs.forEach((tab) => {
    tab.addEventListener("click", () => setTab(tab.dataset.tab));
  });

  els.createForm.addEventListener("submit", onCreateSubmit);
  els.updateForm.addEventListener("submit", onUpdateSubmit);
  els.resetCreate.addEventListener("click", () => {
    els.createForm.reset();
    state.create.env = [];
    renderEnvRows();
    persistState();
  });
  els.resetUpdate.addEventListener("click", () => {
    els.updateForm.reset();
    els.updateForm.querySelector('[name="timeoutSeconds"]').value = "300";
    persistState();
  });
  els.addEnvRow.addEventListener("click", () => {
    state.create.env.push(emptyEnv());
    renderEnvRows();
    persistState();
  });

  document.querySelectorAll("[data-template]").forEach((btn) => {
    btn.addEventListener("click", () => applyTemplate(btn.dataset.template));
  });
}

function hydrateForms() {
  els.createForm.namespace.value = state.create.namespace || "";
  els.createForm.name.value = state.create.name || "";
  els.createForm.image.value = state.create.image || "";
  renderEnvRows();

  els.updateForm.namespace.value = state.update.namespace || "";
  els.updateForm.deployment.value = state.update.deployment || "";
  els.updateForm.container.value = state.update.container || "";
  els.updateForm.image.value = state.update.image || "";
  els.updateForm.timeoutSeconds.value = String(state.update.timeoutSeconds || 300);
  els.updateForm.dryRun.checked = !!state.update.dryRun;
  els.updateForm.wait.checked = !!state.update.wait;
}

function setTab(name) {
  state.activeTab = name;
  render();
  persistState();
}

function render() {
  els.tabs.forEach((tab) => {
    tab.classList.toggle("active", tab.dataset.tab === state.activeTab);
  });
  els.panes.forEach((pane) => {
    pane.classList.toggle("active", pane.dataset.pane === state.activeTab);
  });
  renderTargets();
  renderResults();
  renderEnvRows();
}

function renderTargets() {
  els.recentTargets.innerHTML = "";
  if (!state.recentTargets.length) {
    els.recentTargets.innerHTML = `<div class="subtle">No targets yet.</div>`;
    return;
  }

  state.recentTargets.forEach((item) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "chip ghost";
    button.textContent = `${item.kind}: ${item.namespace}/${item.name}`;
    button.addEventListener("click", () => applyTarget(item));
    els.recentTargets.appendChild(button);
  });
}

function renderResults() {
  els.recentResults.innerHTML = "";
  if (!state.recentResults.length) {
    els.recentResults.innerHTML = `<div class="subtle">No responses yet.</div>`;
    return;
  }

  state.recentResults.slice(0, 6).forEach((item) => {
    const div = document.createElement("div");
    div.className = `result-item ${item.ok ? "ok" : "error"}`;
    div.textContent = `${item.title}: ${item.message}`;
    els.recentResults.appendChild(div);
  });
}

function renderEnvRows() {
  els.envList.innerHTML = "";
  if (!state.create.env.length) {
    state.create.env.push(emptyEnv());
  }

  state.create.env.forEach((env, index) => {
    const row = document.createElement("div");
    row.className = "env-row";
    row.innerHTML = `
      <label class="field">
        <span>Name</span>
        <input data-env-name="${index}" value="${escapeAttr(env.name)}" placeholder="APP_ENV" />
      </label>
      <label class="field env-kind">
        <span>Source</span>
        <select data-env-kind="${index}">
          <option value="value">Literal value</option>
          <option value="secret">Secret key</option>
        </select>
      </label>
      <label class="field">
        <span>Value / Secret name</span>
        <input data-env-value="${index}" placeholder="prod or nginx-secret" />
      </label>
      <label class="field">
        <span>Secret key</span>
        <input data-env-key="${index}" placeholder="database-url" />
      </label>
    `;
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "button secondary";
    remove.textContent = "Remove";
    remove.addEventListener("click", () => {
      state.create.env.splice(index, 1);
      renderEnvRows();
      persistState();
    });
    row.appendChild(remove);

    els.envList.appendChild(row);

    const kind = row.querySelector(`[data-env-kind="${index}"]`);
    const name = row.querySelector(`[data-env-name="${index}"]`);
    const value = row.querySelector(`[data-env-value="${index}"]`);
    const key = row.querySelector(`[data-env-key="${index}"]`);

    kind.value = env.kind;
    name.value = env.name;
    value.value = env.kind === "secret" ? env.secretName : env.value;
    key.value = env.kind === "secret" ? env.secretKey : "";
    key.disabled = env.kind !== "secret";

    kind.addEventListener("change", () => {
      env.kind = kind.value;
      if (env.kind === "secret" && !env.secretName) {
        env.secretName = env.value;
      }
      if (env.kind === "value" && !env.value) {
        env.value = env.secretName;
      }
      renderEnvRows();
      persistState();
    });
    name.addEventListener("input", () => {
      env.name = name.value;
      persistState();
    });
    value.addEventListener("input", () => {
      if (env.kind === "secret") {
        env.secretName = value.value;
      } else {
        env.value = value.value;
      }
      persistState();
    });
    key.addEventListener("input", () => {
      env.secretKey = key.value;
      persistState();
    });
  });

  if (state.create.env.length === 1 && !state.create.env[0].name && !state.create.env[0].value) {
    const firstRow = els.envList.querySelector(".env-row");
    if (firstRow) {
      firstRow.querySelector('[data-env-name="0"]').placeholder = "APP_ENV";
    }
  }
}

function onCreateSubmit(event) {
  event.preventDefault();
  clearErrors(els.createForm);

  const payload = {
    namespace: els.createForm.namespace.value.trim(),
    name: els.createForm.name.value.trim(),
    image: els.createForm.image.value.trim(),
    env: buildCreateEnvPayload(),
  };

  const errors = validateCreate(payload);
  if (Object.keys(errors).length) {
    showErrors(els.createForm, errors);
    return;
  }

  state.create = {
    namespace: payload.namespace,
    name: payload.name,
    image: payload.image,
    env: state.create.env,
  };
  persistState();
  submitJson("/api/v1/deployments", payload, {
    title: "Create deployment",
    onSuccess: (body) => {
      pushTarget({ kind: "create", namespace: body.namespace, name: body.name, image: body.image });
      pushResult(true, "Create deployment", `${body.namespace}/${body.name} created with ${body.image}`);
      els.resultBody.textContent = JSON.stringify(body, null, 2);
    },
  });
}

function onUpdateSubmit(event) {
  event.preventDefault();
  clearErrors(els.updateForm);

  const payload = {
    namespace: els.updateForm.namespace.value.trim(),
    deployment: els.updateForm.deployment.value.trim(),
    container: els.updateForm.container.value.trim(),
    image: els.updateForm.image.value.trim(),
    dryRun: els.updateForm.dryRun.checked,
    wait: els.updateForm.wait.checked,
    timeoutSeconds: Number(els.updateForm.timeoutSeconds.value || "300"),
  };

  const errors = validateUpdate(payload);
  if (Object.keys(errors).length) {
    showErrors(els.updateForm, errors);
    return;
  }

  state.update = payload;
  persistState();
  submitJson("/api/v1/deployments/image", payload, {
    title: "Update image",
    onSuccess: (body) => {
      pushTarget({ kind: "update", namespace: body.namespace, name: body.deployment, image: body.newImage });
      pushResult(true, "Update image", `${body.namespace}/${body.deployment}/${body.container} -> ${body.newImage}`);
      els.resultBody.textContent = JSON.stringify(body, null, 2);
    },
  });
}

function submitJson(url, payload, options) {
  els.connection.textContent = "Submitting request...";
  fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  })
    .then(async (response) => {
      const text = await response.text();
      let body = {};
      try {
        body = text ? JSON.parse(text) : {};
      } catch {
        body = { error: text };
      }
      if (!response.ok) {
        const message = body.error || response.statusText;
        pushResult(false, options.title, message);
        els.resultBody.textContent = JSON.stringify(body, null, 2);
        els.connection.textContent = `API error: ${message}`;
        return;
      }
      options.onSuccess(body);
      els.connection.textContent = "Connected";
    })
    .catch((error) => {
      pushResult(false, options.title, error.message);
      els.resultBody.textContent = error.stack || error.message;
      els.connection.textContent = "Request failed";
    });
}

function validateCreate(payload) {
  const errors = {};
  if (!payload.namespace) errors.namespace = "Namespace is required.";
  if (!payload.name) errors.name = "Name is required.";
  if (!payload.image) errors.image = "Image is required.";
  payload.env.forEach((env, index) => {
    if (!env.name) errors[`env-${index}-name`] = "Env name is required.";
    if (env.kind === "secret") {
      if (!env.secretName) errors[`env-${index}-secretName`] = "Secret name is required.";
      if (!env.secretKey) errors[`env-${index}-secretKey`] = "Secret key is required.";
    } else if (!env.value) {
      errors[`env-${index}-value`] = "Env value is required.";
    }
  });
  return errors;
}

function validateUpdate(payload) {
  const errors = {};
  if (!payload.namespace) errors.namespace = "Namespace is required.";
  if (!payload.deployment) errors.deployment = "Deployment is required.";
  if (!payload.container) errors.container = "Container is required.";
  if (!payload.image) errors.image = "Image is required.";
  return errors;
}

function buildCreateEnvPayload() {
  return state.create.env
    .filter((env) => env.name || env.value || env.secretName || env.secretKey)
    .map((env) => {
      if (env.kind === "secret") {
        return {
          name: env.name.trim(),
          secret: { name: env.secretName.trim(), key: env.secretKey.trim() },
        };
      }
      return { name: env.name.trim(), value: env.value.trim() };
    });
}

function showErrors(form, errors) {
  Object.entries(errors).forEach(([key, message]) => {
    const envMatch = key.match(/^env-(\d+)-(name|value|secretName|secretKey)$/);
    if (envMatch) {
      const index = envMatch[1];
      const part = envMatch[2];
      const host = form.querySelector(`[data-env-${part === "secretName" ? "value" : part === "secretKey" ? "key" : part}="${index}"]`);
      if (host) {
        const error = document.createElement("div");
        error.className = "field-error";
        error.textContent = message;
        host.closest(".field").appendChild(error);
        return;
      }
    }
    const host = form.querySelector(`[name="${key}"]`);
    if (host) {
      const error = document.createElement("div");
      error.className = "field-error";
      error.textContent = message;
      host.closest(".field").appendChild(error);
    }
  });
}

function clearErrors(form) {
  form.querySelectorAll(".field-error").forEach((node) => node.remove());
}

function applyTarget(item) {
  if (item.kind === "create") {
    setTab("create");
    els.createForm.namespace.value = item.namespace;
    els.createForm.name.value = item.name;
    els.createForm.image.value = item.image || els.createForm.image.value;
  } else {
    setTab("update");
    els.updateForm.namespace.value = item.namespace;
    els.updateForm.deployment.value = item.name;
    if (item.image) {
      els.updateForm.image.value = item.image;
    }
  }
  persistState();
}

function applyTemplate(name) {
  if (name === "create-basic") {
    setTab("create");
    els.createForm.namespace.value = "default";
    els.createForm.name.value = "nginx";
    els.createForm.image.value = "nginx:1.27.0";
    state.create.env = [emptyEnv()];
  }
  if (name === "create-secret") {
    setTab("create");
    els.createForm.namespace.value = "default";
    els.createForm.name.value = "api";
    els.createForm.image.value = "ghcr.io/example/api:latest";
    state.create.env = [{
      kind: "secret",
      name: "DATABASE_URL",
      value: "",
      secretName: "api-secret",
      secretKey: "database-url",
    }];
  }
  if (name === "update-basic") {
    setTab("update");
    els.updateForm.namespace.value = "default";
    els.updateForm.deployment.value = "nginx";
    els.updateForm.container.value = "app";
    els.updateForm.image.value = "nginx:1.28.0";
  }
  renderEnvRows();
  persistState();
}

function pushTarget(target) {
  state.recentTargets = [target, ...state.recentTargets.filter((item) => item.namespace !== target.namespace || item.name !== target.name)].slice(0, 8);
  persistState();
  renderTargets();
}

function pushResult(ok, title, message) {
  state.recentResults = [{ ok, title, message }, ...state.recentResults].slice(0, 8);
  persistState();
  renderResults();
}

function pingApi() {
  fetch("/healthz")
    .then((r) => {
      els.connection.textContent = r.ok ? "Connected" : `API returned ${r.status}`;
    })
    .catch(() => {
      els.connection.textContent = "Offline";
    });
}

function emptyEnv() {
  return { kind: "value", name: "", value: "", secretName: "", secretKey: "" };
}

function loadState() {
  const raw = localStorage.getItem(stateKey);
  const defaults = {
    activeTab: "create",
    create: { namespace: "", name: "", image: "", env: [emptyEnv()] },
    update: { namespace: "", deployment: "", container: "", image: "", dryRun: false, wait: false, timeoutSeconds: 300 },
    recentTargets: JSON.parse(localStorage.getItem(recentTargetsKey) || "[]"),
    recentResults: JSON.parse(localStorage.getItem(recentResultsKey) || "[]"),
  };
  if (!raw) return defaults;
  try {
    const parsed = JSON.parse(raw);
    return { ...defaults, ...parsed };
  } catch {
    return defaults;
  }
}

function persistState() {
  localStorage.setItem(stateKey, JSON.stringify(state));
  localStorage.setItem(recentTargetsKey, JSON.stringify(state.recentTargets));
  localStorage.setItem(recentResultsKey, JSON.stringify(state.recentResults));
}

function escapeAttr(value) {
  return String(value).replaceAll("&", "&amp;").replaceAll('"', "&quot;").replaceAll("<", "&lt;").replaceAll(">", "&gt;");
}
