const $ = (id) => document.getElementById(id);

async function api(path, options = {}) {
  const response = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...options
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || `Erro HTTP ${response.status}`);
  return data;
}

function setBusy(busy) {
  $("startButton").disabled = busy;
  $("cancelButton").disabled = !busy;
}

$("startButton").addEventListener("click", async () => {
  const request = {
    sourceDir: $("sourceDir").value.trim(),
    outputDir: $("outputDir").value.trim(),
    dpi: Number($("dpi").value),
    organization: $("organization").value,
    preserveFolders: $("preserveFolders").checked
  };

  try {
    setBusy(true);
    $("statusText").textContent = "Enviando solicitação...";
    $("report").textContent = "";
    await api("/api/convert", { method: "POST", body: JSON.stringify(request) });
  } catch (error) {
    $("statusText").textContent = "Conversão ainda não disponível";
    $("report").textContent = error.message;
  } finally {
    setBusy(false);
  }
});

$("cancelButton").addEventListener("click", async () => {
  try {
    await api("/api/cancel", { method: "POST", body: "{}" });
    $("statusText").textContent = "Cancelamento solicitado";
  } catch (error) {
    $("report").textContent = error.message;
  }
});

async function refreshStatus() {
  try {
    const status = await api("/api/status");
    $("statusText").textContent = status.message || "Aguardando conversão";
    const percent = status.totalPages > 0
      ? Math.round(status.processedPages / status.totalPages * 100)
      : 0;
    $("progress").value = percent;
    $("percent").textContent = `${percent}%`;
    setBusy(Boolean(status.running));
  } catch (_) {
    // O servidor pode ainda estar iniciando.
  }
}
setInterval(refreshStatus, 1000);
refreshStatus();
