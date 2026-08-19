import { useState, useEffect } from "react";
import { Shield, Key, MessageSquare, CheckCircle, XCircle, RefreshCw, Power } from "lucide-react";
import { toast } from "sonner";

interface Instance {
  id: string;
  display_name: string;
  phone: string;
  status: string;
  max_concurrent_calls: number;
  proxy_id: string;
  chatseguro_inbox_id: string;
}

export const LinesPage = () => {
  const [instances, setInstances] = useState<Instance[]>([]);
  const [loading, setLoading] = useState(true);
  
  // Modals/Forms State
  const [selectedInst, setSelectedInst] = useState<Instance | null>(null);
  const [showProxyModal, setShowProxyModal] = useState(false);
  const [showCSModal, setShowCSModal] = useState(false);
  const [showCapacityModal, setShowCapacityModal] = useState(false);

  const [proxyForm, setProxyForm] = useState({
    type: "socks5",
    host: "",
    port: 1080,
    username: "",
    secret: ""
  });

  const [csForm, setCsForm] = useState({
    url: "https://chatseguro.io",
    account_id: 1,
    account_token: "",
    inbox_id: 1
  });

  const [capacity, setCapacity] = useState(8);

  const tenantID = "tenant-A"; // Mock do tenant autenticado

  const fetchInstances = async () => {
    try {
      setLoading(true);
      const res = await fetch("/api/instances", {
        headers: {
          "X-Tenant-ID": tenantID,
          "X-API-Key": "tenant-A" // Mock da chave de tenant autenticado
        }
      });
      const data = await res.json();
      setInstances(data.instances || []);
    } catch (e) {
      toast.error("Erro ao carregar linhas");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchInstances();
  }, []);

  const handleTestProxy = async (id: string) => {
    toast.info("Testando conectividade de rede do proxy...");
    try {
      const res = await fetch(`/api/instances/${id}/proxy/test`, {
        method: "POST",
        headers: {
          "X-Tenant-ID": tenantID,
          "X-API-Key": "tenant-A"
        }
      });
      const data = await res.json();
      if (data.status === "HEALTHY") {
        toast.success(`Proxy saudável! IP de saída: ${data.public_ip} (Latência: ${data.latency_ms}ms)`);
      } else {
        toast.error("Proxy indisponível para conexão");
      }
    } catch (e) {
      toast.error("Erro ao tentar conectar ao proxy");
    }
  };

  const handleSaveProxy = async () => {
    if (!selectedInst) return;
    try {
      const res = await fetch(`/api/instances/${selectedInst.id}/proxy`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Tenant-ID": tenantID,
          "X-API-Key": "tenant-A"
        },
        body: JSON.stringify(proxyForm)
      });
      if (res.ok) {
        toast.success("Proxy configurado com sucesso");
        setShowProxyModal(false);
        fetchInstances();
      } else {
        toast.error("Falha ao salvar proxy");
      }
    } catch (e) {
      toast.error("Erro ao enviar dados do proxy");
    }
  };

  const handleSaveChatSeguro = async () => {
    if (!selectedInst) return;
    try {
      const res = await fetch(`/api/instances/${selectedInst.id}/chatseguro`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Tenant-ID": tenantID,
          "X-API-Key": "tenant-A"
        },
        body: JSON.stringify(csForm)
      });
      if (res.ok) {
        toast.success("Inbox ChatSeguro vinculada com sucesso");
        setShowCSModal(false);
        fetchInstances();
      } else {
        toast.error("Falha ao vincular inbox");
      }
    } catch (e) {
      toast.error("Erro ao enviar dados do ChatSeguro");
    }
  };

  const handleUpdateCapacity = async () => {
    if (!selectedInst) return;
    // Simula salvamento de capacidade
    toast.success("Capacidade de chamadas simultâneas atualizada");
    setShowCapacityModal(false);
    fetchInstances();
  };

  const handleLogout = async (id: string) => {
    if (!confirm("Deseja desconectar esta linha?")) return;
    try {
      const res = await fetch(`/api/instances/${id}/logout`, {
        method: "POST",
        headers: {
          "X-Tenant-ID": tenantID,
          "X-API-Key": "tenant-A"
        }
      });
      if (res.ok) {
        toast.success("Linha desconectada");
        fetchInstances();
      }
    } catch (e) {
      toast.error("Erro ao desconectar linha");
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-slate-900 dark:text-white">Gerenciar Linhas</h1>
          <p className="text-sm text-slate-500">Controle de chip WhatsApp, pareamento, proxy anti-ban e caixas de entrada.</p>
        </div>
        <button 
          onClick={fetchInstances}
          className="flex items-center gap-2 px-3 py-1.5 text-sm bg-white dark:bg-slate-800 border rounded-lg hover:bg-slate-50"
        >
          <RefreshCw className="h-4 w-4" /> Atualizar
        </button>
      </div>

      {loading ? (
        <div className="text-center py-12">Carregando linhas...</div>
      ) : instances.length === 0 ? (
        <div className="text-center py-12 border border-dashed rounded-xl bg-white dark:bg-slate-800">
          <p className="text-slate-500">Nenhuma linha cadastrada para este tenant.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {instances.map((inst) => (
            <div key={inst.id} className="bg-white dark:bg-slate-900 border rounded-xl shadow-sm p-5 space-y-4">
              <div className="flex justify-between items-start">
                <div>
                  <h3 className="font-semibold text-slate-800 dark:text-white">{inst.display_name}</h3>
                  <p className="text-xs text-slate-500 font-mono">{inst.phone || "Sem número vinculado"}</p>
                </div>
                <span className={`px-2 py-0.5 text-xs font-semibold rounded-full flex items-center gap-1 ${
                  inst.status === "CONNECTED" ? "bg-emerald-50 text-emerald-700 border border-emerald-200" :
                  inst.status === "BUSY" ? "bg-amber-50 text-amber-700 border border-amber-200" :
                  "bg-slate-50 text-slate-700 border border-slate-200"
                }`}>
                  {inst.status === "CONNECTED" || inst.status === "BUSY" ? <CheckCircle className="h-3 w-3" /> : <XCircle className="h-3 w-3" />}
                  {inst.status}
                </span>
              </div>

              {/* Status Details */}
              <div className="grid grid-cols-3 gap-2 py-2 border-y text-center text-xs">
                <div>
                  <span className="block text-slate-400">Calls</span>
                  <strong className="text-slate-700 dark:text-slate-200">0 / {inst.max_concurrent_calls}</strong>
                </div>
                <div>
                  <span className="block text-slate-400">Proxy</span>
                  <strong className={inst.proxy_id ? "text-emerald-600" : "text-slate-400"}>
                    {inst.proxy_id ? "Ativo" : "Nenhum"}
                  </strong>
                </div>
                <div>
                  <span className="block text-slate-400">ChatSeguro</span>
                  <strong className={inst.chatseguro_inbox_id ? "text-emerald-600" : "text-slate-400"}>
                    {inst.chatseguro_inbox_id ? "Linkado" : "Nenhum"}
                  </strong>
                </div>
              </div>

              {/* Action Buttons */}
              <div className="grid grid-cols-2 gap-2 text-xs">
                <button 
                  onClick={() => { setSelectedInst(inst); setShowProxyModal(true); }}
                  className="flex items-center justify-center gap-1.5 py-1.5 px-3 bg-slate-50 dark:bg-slate-800 hover:bg-slate-100 rounded-lg"
                >
                  <Shield className="h-3.5 w-3.5" /> Proxy
                </button>
                <button 
                  onClick={() => { setSelectedInst(inst); setShowCSModal(true); }}
                  className="flex items-center justify-center gap-1.5 py-1.5 px-3 bg-slate-50 dark:bg-slate-800 hover:bg-slate-100 rounded-lg"
                >
                  <MessageSquare className="h-3.5 w-3.5" /> ChatSeguro
                </button>
                <button 
                  onClick={() => { setSelectedInst(inst); setShowCapacityModal(true); }}
                  className="flex items-center justify-center gap-1.5 py-1.5 px-3 bg-slate-50 dark:bg-slate-800 hover:bg-slate-100 rounded-lg"
                >
                  <Key className="h-3.5 w-3.5" /> Capacidade
                </button>
                {inst.status === "CONNECTED" && (
                  <button 
                    onClick={() => handleLogout(inst.id)}
                    className="flex items-center justify-center gap-1.5 py-1.5 px-3 bg-rose-50 hover:bg-rose-100 text-rose-700 rounded-lg"
                  >
                    <Power className="h-3.5 w-3.5" /> Logout
                  </button>
                )}
                {inst.proxy_id && (
                  <button 
                    onClick={() => handleTestProxy(inst.id)}
                    className="col-span-2 py-1.5 border border-emerald-200 hover:bg-emerald-50 text-emerald-700 rounded-lg font-medium"
                  >
                    Testar Proxy Conectado
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Proxy Settings Modal */}
      {showProxyModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
          <div className="bg-white dark:bg-slate-900 border p-6 rounded-xl w-full max-w-md space-y-4">
            <h3 className="font-bold text-lg">Configurar Proxy (Anti-Ban)</h3>
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-semibold mb-1">Tipo</label>
                <select 
                  value={proxyForm.type}
                  onChange={(e) => setProxyForm({ ...proxyForm, type: e.target.value })}
                  className="w-full border rounded-lg p-2 text-sm bg-transparent"
                >
                  <option value="socks5">SOCKS5</option>
                  <option value="http">HTTP</option>
                  <option value="https">HTTPS</option>
                </select>
              </div>
              <div>
                <label className="block text-xs font-semibold mb-1">Host/IP</label>
                <input 
                  type="text" 
                  value={proxyForm.host}
                  onChange={(e) => setProxyForm({ ...proxyForm, host: e.target.value })}
                  className="w-full border rounded-lg p-2 text-sm bg-transparent"
                  placeholder="proxy.exemplo.com"
                />
              </div>
              <div className="grid grid-cols-3 gap-2">
                <div className="col-span-1">
                  <label className="block text-xs font-semibold mb-1">Porta</label>
                  <input 
                    type="number" 
                    value={proxyForm.port}
                    onChange={(e) => setProxyForm({ ...proxyForm, port: parseInt(e.target.value) || 1080 })}
                    className="w-full border rounded-lg p-2 text-sm bg-transparent"
                  />
                </div>
                <div className="col-span-2">
                  <label className="block text-xs font-semibold mb-1">Usuário</label>
                  <input 
                    type="text" 
                    value={proxyForm.username}
                    onChange={(e) => setProxyForm({ ...proxyForm, username: e.target.value })}
                    className="w-full border rounded-lg p-2 text-sm bg-transparent"
                  />
                </div>
              </div>
              <div>
                <label className="block text-xs font-semibold mb-1">Senha/Secret</label>
                <input 
                  type="password" 
                  value={proxyForm.secret}
                  onChange={(e) => setProxyForm({ ...proxyForm, secret: e.target.value })}
                  className="w-full border rounded-lg p-2 text-sm bg-transparent"
                  placeholder="••••••••"
                />
              </div>
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <button onClick={() => setShowProxyModal(false)} className="px-4 py-2 border rounded-lg text-sm">Cancelar</button>
              <button onClick={handleSaveProxy} className="px-4 py-2 bg-slate-900 text-white rounded-lg text-sm">Salvar Proxy</button>
            </div>
          </div>
        </div>
      )}

      {/* ChatSeguro Link Modal */}
      {showCSModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
          <div className="bg-white dark:bg-slate-900 border p-6 rounded-xl w-full max-w-md space-y-4">
            <h3 className="font-bold text-lg">Vincular Inbox ChatSeguro</h3>
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-semibold mb-1">URL da Instância</label>
                <input 
                  type="text" 
                  value={csForm.url}
                  onChange={(e) => setCsForm({ ...csForm, url: e.target.value })}
                  className="w-full border rounded-lg p-2 text-sm bg-transparent"
                />
              </div>
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <label className="block text-xs font-semibold mb-1">Conta ID</label>
                  <input 
                    type="number" 
                    value={csForm.account_id}
                    onChange={(e) => setCsForm({ ...csForm, account_id: parseInt(e.target.value) || 1 })}
                    className="w-full border rounded-lg p-2 text-sm bg-transparent"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold mb-1">Inbox ID</label>
                  <input 
                    type="number" 
                    value={csForm.inbox_id}
                    onChange={(e) => setCsForm({ ...csForm, inbox_id: parseInt(e.target.value) || 1 })}
                    className="w-full border rounded-lg p-2 text-sm bg-transparent"
                  />
                </div>
              </div>
              <div>
                <label className="block text-xs font-semibold mb-1">Token de Acesso</label>
                <input 
                  type="password" 
                  value={csForm.account_token}
                  onChange={(e) => setCsForm({ ...csForm, account_token: e.target.value })}
                  className="w-full border rounded-lg p-2 text-sm bg-transparent"
                  placeholder="••••••••"
                />
              </div>
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <button onClick={() => setShowCSModal(false)} className="px-4 py-2 border rounded-lg text-sm">Cancelar</button>
              <button onClick={handleSaveChatSeguro} className="px-4 py-2 bg-slate-900 text-white rounded-lg text-sm">Vincular</button>
            </div>
          </div>
        </div>
      )}

      {/* Capacity Settings Modal */}
      {showCapacityModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
          <div className="bg-white dark:bg-slate-900 border p-6 rounded-xl w-full max-w-sm space-y-4">
            <h3 className="font-bold text-lg">Definir Capacidade de Chamadas</h3>
            <div>
              <label className="block text-xs font-semibold mb-1">Canais Simultâneos Permitidos</label>
              <input 
                type="number" 
                value={capacity}
                onChange={(e) => setCapacity(parseInt(e.target.value) || 8)}
                className="w-full border rounded-lg p-2 text-sm bg-transparent"
                min="1"
                max="32"
              />
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <button onClick={() => setShowCapacityModal(false)} className="px-4 py-2 border rounded-lg text-sm">Cancelar</button>
              <button onClick={handleUpdateCapacity} className="px-4 py-2 bg-slate-900 text-white rounded-lg text-sm">Salvar</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
