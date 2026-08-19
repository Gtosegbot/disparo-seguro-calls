import { useState } from "react";
import { Play, Pause, Square, Plus, Volume2 } from "lucide-react";
import { toast } from "sonner";

interface Campaign {
  id: string;
  name: string;
  mode: string;
  status: string;
  max_concurrent_calls: number;
  dial_interval_seconds: number;
  max_attempts: number;
  retry_delay_seconds: number;
  voice_profile?: string;
  provider_policy?: string;
}

export const CampaignPage = () => {
  const [campaigns, setCampaigns] = useState<Campaign[]>([
    {
      id: "camp-1",
      name: "Prospecção Premium São Paulo",
      mode: "AI",
      status: "RUNNING",
      max_concurrent_calls: 12,
      dial_interval_seconds: 5,
      max_attempts: 3,
      retry_delay_seconds: 60,
      voice_profile: "sales",
      provider_policy: "Premium"
    },
    {
      id: "camp-2",
      name: "Pesquisa NPS Pós-Venda",
      mode: "AI",
      status: "PAUSED",
      max_concurrent_calls: 8,
      dial_interval_seconds: 3,
      max_attempts: 2,
      retry_delay_seconds: 120,
      voice_profile: "survey",
      provider_policy: "Economy"
    }
  ]);

  const [showBuilder, setShowBuilder] = useState(false);
  const [form, setForm] = useState({
    name: "",
    mode: "AI",
    max_concurrent_calls: 8,
    dial_interval_seconds: 5,
    max_attempts: 3,
    retry_delay_seconds: 60,
    voice_profile: "sales",
    provider_policy: "Balanced"
  });

  const handleStart = (id: string) => {
    setCampaigns(campaigns.map(c => c.id === id ? { ...c, status: "RUNNING" } : c));
    toast.success("Campanha iniciada com sucesso");
  };

  const handlePause = (id: string) => {
    setCampaigns(campaigns.map(c => c.id === id ? { ...c, status: "PAUSED" } : c));
    toast.info("Campanha pausada (novos disparos bloqueados)");
  };

  const handleDrain = (id: string) => {
    setCampaigns(campaigns.map(c => c.id === id ? { ...c, status: "DRAINING" } : c));
    toast.info("Campanha entrando em esgotamento (draining)");
  };

  const handleStop = (id: string) => {
    setCampaigns(campaigns.map(c => c.id === id ? { ...c, status: "STOPPED" } : c));
    toast.error("Campanha encerrada");
  };

  const handleCreate = () => {
    const newCamp = {
      id: "camp-" + (campaigns.length + 1),
      ...form,
      status: "READY"
    };
    setCampaigns([...campaigns, newCamp]);
    setShowBuilder(false);
    toast.success("Campanha criada e pronta para disparo");
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-slate-900 dark:text-white">Campanhas (Dialer)</h1>
          <p className="text-sm text-slate-500">Cadencie disparos de chamadas manuais (H2H) ou automatizadas por Voz de IA (DS Voice).</p>
        </div>
        <button 
          onClick={() => setShowBuilder(true)}
          className="flex items-center gap-2 px-4 py-2 bg-slate-900 hover:bg-slate-800 text-white rounded-lg text-sm"
        >
          <Plus className="h-4 w-4" /> Nova Campanha
        </button>
      </div>

      <div className="grid grid-cols-1 gap-6">
        {campaigns.map((camp) => (
          <div key={camp.id} className="bg-white dark:bg-slate-900 border rounded-xl p-5 flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
            <div className="space-y-1">
              <div className="flex items-center gap-2">
                <h3 className="font-semibold text-slate-800 dark:text-white">{camp.name}</h3>
                <span className={`px-2 py-0.5 text-xs font-semibold rounded-full ${
                  camp.mode === "AI" ? "bg-indigo-50 text-indigo-700 border border-indigo-200" : "bg-teal-50 text-teal-700 border border-teal-200"
                }`}>
                  {camp.mode === "AI" ? "DS Voice IA" : "Human H2H"}
                </span>
                <span className={`px-2 py-0.5 text-xs font-semibold rounded-full ${
                  camp.status === "RUNNING" ? "bg-emerald-50 text-emerald-700 border border-emerald-200" :
                  camp.status === "PAUSED" ? "bg-amber-50 text-amber-700 border border-amber-200" :
                  camp.status === "DRAINING" ? "bg-sky-50 text-sky-700 border border-sky-200" :
                  "bg-slate-100 text-slate-700"
                }`}>
                  {camp.status}
                </span>
              </div>
              <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-500">
                <span>Canais simultâneos: <strong>{camp.max_concurrent_calls}</strong></span>
                <span>Intervalo por linha: <strong>{camp.dial_interval_seconds}s</strong></span>
                {camp.mode === "AI" && (
                  <>
                    <span>Perfil de voz: <strong>{camp.voice_profile}</strong></span>
                    <span>Provedor: <strong>{camp.provider_policy}</strong></span>
                  </>
                )}
              </div>
            </div>

            {/* Campaign Controls */}
            <div className="flex items-center gap-2 text-xs">
              {camp.status !== "RUNNING" ? (
                <button 
                  onClick={() => handleStart(camp.id)}
                  className="flex items-center gap-1 py-1.5 px-3 bg-emerald-50 hover:bg-emerald-100 text-emerald-700 rounded-lg"
                >
                  <Play className="h-3.5 w-3.5" /> Iniciar
                </button>
              ) : (
                <>
                  <button 
                    onClick={() => handlePause(camp.id)}
                    className="flex items-center gap-1 py-1.5 px-3 bg-amber-50 hover:bg-amber-100 text-amber-700 rounded-lg"
                  >
                    <Pause className="h-3.5 w-3.5" /> Pausar
                  </button>
                  <button 
                    onClick={() => handleDrain(camp.id)}
                    className="flex items-center gap-1 py-1.5 px-3 bg-sky-50 hover:bg-sky-100 text-sky-700 rounded-lg"
                  >
                    Esgotar (Drain)
                  </button>
                </>
              )}
              {camp.status !== "STOPPED" && (
                <button 
                  onClick={() => handleStop(camp.id)}
                  className="flex items-center gap-1 py-1.5 px-3 bg-rose-50 hover:bg-rose-100 text-rose-700 rounded-lg"
                >
                  <Square className="h-3.5 w-3.5" /> Parar
                </button>
              )}
            </div>
          </div>
        ))}
      </div>

      {/* Campaign Builder Modal */}
      {showBuilder && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50 overflow-y-auto">
          <div className="bg-white dark:bg-slate-900 border p-6 rounded-xl w-full max-w-lg space-y-4 my-8">
            <h3 className="font-bold text-lg">Criar Nova Campanha (Builder)</h3>
            
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-semibold mb-1">Nome da Campanha</label>
                <input 
                  type="text" 
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  className="w-full border rounded-lg p-2 text-sm bg-transparent"
                  placeholder="Ex: Prospecção Leads Evento"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-semibold mb-1">Modo de Operação</label>
                  <select 
                    value={form.mode}
                    onChange={(e) => setForm({ ...form, mode: e.target.value })}
                    className="w-full border rounded-lg p-2 text-sm bg-transparent"
                  >
                    <option value="AI">DS Voice IA (Voz em tempo real)</option>
                    <option value="H2H">VoIP Humano (H2H)</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-semibold mb-1">Canais Simultâneos Max</label>
                  <input 
                    type="number" 
                    value={form.max_concurrent_calls}
                    onChange={(e) => setForm({ ...form, max_concurrent_calls: parseInt(e.target.value) || 8 })}
                    className="w-full border rounded-lg p-2 text-sm bg-transparent"
                  />
                </div>
              </div>

              <div className="grid grid-cols-3 gap-2">
                <div>
                  <label className="block text-xs font-semibold mb-1">Intervalo por linha (s)</label>
                  <input 
                    type="number" 
                    value={form.dial_interval_seconds}
                    onChange={(e) => setForm({ ...form, dial_interval_seconds: parseInt(e.target.value) || 5 })}
                    className="w-full border rounded-lg p-2 text-sm bg-transparent"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold mb-1">Max Tentativas</label>
                  <input 
                    type="number" 
                    value={form.max_attempts}
                    onChange={(e) => setForm({ ...form, max_attempts: parseInt(e.target.value) || 3 })}
                    className="w-full border rounded-lg p-2 text-sm bg-transparent"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold mb-1">Atraso Retry (s)</label>
                  <input 
                    type="number" 
                    value={form.retry_delay_seconds}
                    onChange={(e) => setForm({ ...form, retry_delay_seconds: parseInt(e.target.value) || 60 })}
                    className="w-full border rounded-lg p-2 text-sm bg-transparent"
                  />
                </div>
              </div>

              {/* Seção de IA exposta condicionalmente */}
              {form.mode === "AI" && (
                <div className="p-4 border rounded-xl bg-slate-50 dark:bg-slate-800/50 space-y-3">
                  <div className="flex items-center gap-1.5 text-indigo-700 font-semibold text-xs uppercase tracking-wider">
                    <Volume2 className="h-4 w-4" /> Configurações de Voz DS Voice
                  </div>

                  <div className="grid grid-cols-2 gap-2">
                    <div>
                      <label className="block text-xs font-semibold mb-1">Perfil de Voz</label>
                      <select 
                        value={form.voice_profile}
                        onChange={(e) => setForm({ ...form, voice_profile: e.target.value })}
                        className="w-full border rounded-lg p-2 text-sm bg-transparent"
                      >
                        <option value="sales">Vendas (Sales)</option>
                        <option value="survey">Pesquisa (Survey)</option>
                        <option value="support">Suporte (Support)</option>
                        <option value="dynamic_interviewer">Entrevistador Dinâmico</option>
                      </select>
                    </div>
                    <div>
                      <label className="block text-xs font-semibold mb-1">Política de Provedor</label>
                      <select 
                        value={form.provider_policy}
                        onChange={(e) => setForm({ ...form, provider_policy: e.target.value })}
                        className="w-full border rounded-lg p-2 text-sm bg-transparent"
                      >
                        <option value="Balanced">Balanced (Custo x Latência)</option>
                        <option value="Economy">Economy (Tiers gratuitos/baratos)</option>
                        <option value="Low Latency">Low Latency (Fast responses)</option>
                        <option value="Premium">Premium Realtime (Grok/Gemini Live)</option>
                      </select>
                    </div>
                  </div>
                </div>
              )}
            </div>

            <div className="flex justify-end gap-2 pt-2">
              <button onClick={() => setShowBuilder(false)} className="px-4 py-2 border rounded-lg text-sm">Cancelar</button>
              <button onClick={handleCreate} className="px-4 py-2 bg-slate-900 text-white rounded-lg text-sm">Criar Campanha</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
