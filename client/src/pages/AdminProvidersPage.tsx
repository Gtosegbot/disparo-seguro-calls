import { useState, useEffect } from "react";
import { CheckCircle, XCircle, RefreshCw, Save, Sliders } from "lucide-react";
import { toast } from "sonner";

interface ProviderCatalogItem {
  name: string;
  model: string;
  provider_type: string;
  estimated_cost: number;
  latency_target: number;
  quality_class: string;
  license: string;
  enabled: boolean;
  health: number;
  priority: number;
  weight: number;
  active_sessions: number;
  success_rate: number;
  avg_ttfb_ms: number;
}

export const AdminProvidersPage = () => {
  const [providers, setProviders] = useState<ProviderCatalogItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedProv, setSelectedProv] = useState<ProviderCatalogItem | null>(null);

  const [form, setForm] = useState({
    enabled: true,
    priority: 1,
    weight: 50
  });

  const fetchProviders = async () => {
    try {
      setLoading(true);
      const res = await fetch("/api/admin/providers");
      const data = await res.json();
      setProviders(data.providers || []);
    } catch (e) {
      toast.error("Erro ao carregar catálogo de provedores");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchProviders();
  }, []);

  const handleEdit = (prov: ProviderCatalogItem) => {
    setSelectedProv(prov);
    setForm({
      enabled: prov.enabled,
      priority: prov.priority,
      weight: prov.weight
    });
  };

  const handleSave = async () => {
    if (!selectedProv) return;
    try {
      const res = await fetch(`/api/admin/providers/${selectedProv.name}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify(form)
      });
      if (res.ok) {
        toast.success("Configurações do provedor atualizadas");
        setSelectedProv(null);
        fetchProviders();
      } else {
        toast.error("Erro ao salvar configurações");
      }
    } catch (e) {
      toast.error("Erro ao processar requisição");
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-slate-900 dark:text-white">Admin Providers (DS Voice)</h1>
          <p className="text-sm text-slate-500">Mapeamento de prioridades, controle de custos e governança do pool de provedores de voz de IA.</p>
        </div>
        <button 
          onClick={fetchProviders}
          className="flex items-center gap-2 px-3 py-1.5 text-sm bg-white dark:bg-slate-800 border rounded-lg hover:bg-slate-50"
        >
          <RefreshCw className="h-4 w-4" /> Atualizar
        </button>
      </div>

      {loading ? (
        <div className="text-center py-12">Carregando catálogo de provedores...</div>
      ) : (
        <div className="grid grid-cols-1 gap-6">
          {providers.map((prov) => (
            <div key={prov.name} className="bg-white dark:bg-slate-900 border rounded-xl p-5 flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
              <div className="space-y-2 flex-1">
                <div className="flex items-center gap-2">
                  <h3 className="font-bold text-slate-800 dark:text-white">{prov.name}</h3>
                  <span className="text-xs font-mono text-slate-400">({prov.model})</span>
                  <span className={`px-2 py-0.5 text-xs font-semibold rounded-full flex items-center gap-1 ${
                    prov.enabled ? "bg-emerald-50 text-emerald-700 border border-emerald-200" : "bg-slate-100 text-slate-500"
                  }`}>
                    {prov.enabled ? <CheckCircle className="h-3 w-3" /> : <XCircle className="h-3 w-3" />}
                    {prov.enabled ? "Enabled" : "Disabled"}
                  </span>
                </div>

                <div className="grid grid-cols-2 md:grid-cols-5 gap-4 text-xs text-slate-500">
                  <div>
                    <span className="block text-slate-400">Custo Estimado</span>
                    <strong className="text-slate-700 dark:text-slate-200">${prov.estimated_cost.toFixed(2)}/min</strong>
                  </div>
                  <div>
                    <span className="block text-slate-400">TTFB Alvo</span>
                    <strong className="text-slate-700 dark:text-slate-200">{prov.latency_target}ms</strong>
                  </div>
                  <div>
                    <span className="block text-slate-400">Sucesso IA</span>
                    <strong className="text-slate-700 dark:text-slate-200">{(prov.success_rate * 100).toFixed(0)}%</strong>
                  </div>
                  <div>
                    <span className="block text-slate-400">Sessions</span>
                    <strong className="text-slate-700 dark:text-slate-200">{prov.active_sessions} ativas</strong>
                  </div>
                  <div>
                    <span className="block text-slate-400">Pesos / Prioridade</span>
                    <strong className="text-slate-700 dark:text-slate-200">W:{prov.weight}% / P:{prov.priority}</strong>
                  </div>
                </div>
              </div>

              <div>
                <button 
                  onClick={() => handleEdit(prov)}
                  className="flex items-center gap-1.5 py-1.5 px-3 bg-slate-100 hover:bg-slate-200 text-slate-700 rounded-lg text-xs"
                >
                  <Sliders className="h-3.5 w-3.5" /> Ajustar Tiers
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Editor Modal */}
      {selectedProv && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
          <div className="bg-white dark:bg-slate-900 border p-6 rounded-xl w-full max-w-sm space-y-4">
            <h3 className="font-bold text-lg">Ajustar Provedor: {selectedProv.name}</h3>
            
            <div className="space-y-3">
              <div className="flex items-center justify-between border-b pb-2">
                <span className="text-sm font-semibold">Ativado</span>
                <input 
                  type="checkbox" 
                  checked={form.enabled}
                  onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
                  className="h-4 w-4"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold mb-1">Prioridade do Fallback</label>
                <input 
                  type="number" 
                  value={form.priority}
                  onChange={(e) => setForm({ ...form, priority: parseInt(e.target.value) || 1 })}
                  className="w-full border rounded-lg p-2 text-sm bg-transparent"
                  min="1"
                  max="10"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold mb-1">Peso da Distribuição (%)</label>
                <input 
                  type="number" 
                  value={form.weight}
                  onChange={(e) => setForm({ ...form, weight: parseInt(e.target.value) || 0 })}
                  className="w-full border rounded-lg p-2 text-sm bg-transparent"
                  min="0"
                  max="100"
                />
              </div>
            </div>

            <div className="flex justify-end gap-2 pt-2">
              <button onClick={() => setSelectedProv(null)} className="px-4 py-2 border rounded-lg text-sm">Cancelar</button>
              <button onClick={handleSave} className="flex items-center gap-1.5 px-4 py-2 bg-slate-900 text-white rounded-lg text-sm">
                <Save className="h-4 w-4" /> Salvar Alterações
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
