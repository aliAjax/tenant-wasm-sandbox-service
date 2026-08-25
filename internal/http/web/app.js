const state = { token: sessionStorage.getItem("wasm-token") || "dev-secret" };
const byId = (id) => document.getElementById(id);
const tokenInput = byId("token");
tokenInput.value = state.token;

async function request(path, authenticated = true) {
  const headers = authenticated ? { Authorization: `Bearer ${state.token}` } : {};
  const response = await fetch(path, { headers });
  if (!response.ok) throw new Error(`${response.status} ${response.statusText}`);
  return response.json();
}

function showError(message) {
  const toast = byId("toast");
  toast.textContent = message;
  toast.classList.add("show");
  window.setTimeout(() => toast.classList.remove("show"), 3200);
}

function renderExecutions(items) {
  const body = byId("executions");
  if (!items.length) {
    body.innerHTML = '<tr><td colspan="5" class="empty">No executions have been submitted.</td></tr>';
    return;
  }
  body.replaceChildren(...items.map((item) => {
    const row = document.createElement("tr");
    const values = [item.id, item.tenant_id, item.module_id, item.status, item.created_at];
    values.forEach((value, index) => {
      const cell = document.createElement("td");
      if (index === 3) {
        const badge = document.createElement("span");
        badge.className = "status";
        badge.textContent = value || "unknown";
        cell.appendChild(badge);
      } else {
        cell.textContent = value || "-";
      }
      row.appendChild(cell);
    });
    return row;
  }));
}

async function refresh() {
  try {
    const [health, nodes, cache, executions, metrics] = await Promise.all([
      request("/healthz", false), request("/v1/nodes"), request("/v1/cache"),
      request("/v1/executions?limit=50"), fetch("/metrics").then((r) => r.text()),
    ]);
    const node = nodes.items?.[0];
    const queueMatch = metrics.match(/wasm_queue_depth\s+(\d+)/);
    byId("health").className = "health ok";
    byId("health").lastElementChild.textContent = health.status === "ok" ? "Service online" : health.status;
    byId("node-state").textContent = node?.state || "Unknown";
    byId("node-name").textContent = node?.id || "No node reported";
    byId("queue-depth").textContent = queueMatch?.[1] || "0";
    byId("cache-entries").textContent = cache.entries ?? cache.count ?? "0";
    byId("execution-count").textContent = executions.count ?? executions.items?.length ?? "0";
    byId("updated-at").textContent = `Updated ${new Date().toLocaleTimeString()}`;
    renderExecutions(executions.items || []);
  } catch (error) {
    byId("health").className = "health error";
    byId("health").lastElementChild.textContent = "Connection failed";
    showError(`Could not load service data: ${error.message}`);
  }
}

byId("token-form").addEventListener("submit", (event) => {
  event.preventDefault();
  state.token = tokenInput.value.trim();
  sessionStorage.setItem("wasm-token", state.token);
  refresh();
});
byId("refresh").addEventListener("click", refresh);
refresh();
