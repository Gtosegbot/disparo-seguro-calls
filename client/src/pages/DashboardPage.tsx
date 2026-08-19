import { BarChart3, Users, PhoneCall, Clock, DollarSign, Activity } from "lucide-react";

export const DashboardPage = () => {
  // Mock data para estatísticas e custos operacionais
  const stats = {
    queued: 820,
    activeCalls: 12,
    completed: 143,
    busy: 21,
    noAnswer: 54,
    failed: 3,
    retries: 17,
    aiSessions: 11,
    avgTTFBMs: 140,
    avgDurationSec: 42,
    platformCostUSD: 1.45,
    providerCostUSD: 3.12
  };

  const instances = [
    { name: "Linha Principal SP (Vendas)", active: 4, max: 8, status: "CONNECTED" },
    { name: "WhatsApp Backup (NPS)", active: 8, max: 8, status: "BUSY" },
    { name: "WhatsApp SDR 03 (Suporte)", active: 0, max: 8, status: "CONNECTED" }
  ];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-slate-900 dark:text-white">Observabilidade & Métricas</h1>
        <p className="text-sm text-slate-500">Monitoramento em tempo real do tráfego VoIP, andamento de leads e custos de IA.</p>
      </div>

      {/* Grid de Estatísticas */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <div className="bg-white dark:bg-slate-900 border rounded-xl p-5 flex items-center gap-4">
          <div className="p-3 bg-indigo-50 text-indigo-700 rounded-lg">
            <Users className="h-6 w-6" />
          </div>
          <div>
            <span className="block text-xs text-slate-400">Leads em Fila</span>
            <strong className="text-xl text-slate-800 dark:text-white">{stats.queued}</strong>
          </div>
        </div>

        <div className="bg-white dark:bg-slate-900 border rounded-xl p-5 flex items-center gap-4">
          <div className="p-3 bg-emerald-50 text-emerald-700 rounded-lg">
            <PhoneCall className="h-6 w-6" />
          </div>
          <div>
            <span className="block text-xs text-slate-400">Ligações Ativas</span>
            <strong className="text-xl text-slate-800 dark:text-white">{stats.activeCalls}</strong>
          </div>
        </div>

        <div className="bg-white dark:bg-slate-900 border rounded-xl p-5 flex items-center gap-4">
          <div className="p-3 bg-amber-50 text-amber-700 rounded-lg">
            <Clock className="h-6 w-6" />
          </div>
          <div>
            <span className="block text-xs text-slate-400">Latência Média (TTFB)</span>
            <strong className="text-xl text-slate-800 dark:text-white">{stats.avgTTFBMs}ms</strong>
          </div>
        </div>

        <div className="bg-white dark:bg-slate-900 border rounded-xl p-5 flex items-center gap-4">
          <div className="p-3 bg-sky-50 text-sky-700 rounded-lg">
            <DollarSign className="h-6 w-6" />
          </div>
          <div>
            <span className="block text-xs text-slate-400">Custo Total Estimado</span>
            <strong className="text-xl text-slate-800 dark:text-white">
              ${(stats.platformCostUSD + stats.providerCostUSD).toFixed(2)}
            </strong>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Painel de Custos */}
        <div className="bg-white dark:bg-slate-900 border rounded-xl p-5 space-y-4 col-span-1">
          <h3 className="font-semibold text-slate-800 dark:text-white flex items-center gap-2">
            <Activity className="h-4 w-4 text-slate-500" /> Distribuição de Custos
          </h3>
          <div className="space-y-3">
            <div className="flex justify-between border-b pb-2 text-sm">
              <span className="text-slate-500">Custo Plataforma (DS Voice)</span>
              <strong className="text-slate-800 dark:text-slate-200">${stats.platformCostUSD.toFixed(2)}</strong>
            </div>
            <div className="flex justify-between border-b pb-2 text-sm">
              <span className="text-slate-500">Custo API Provedor (Grok/Gemini)</span>
              <strong className="text-slate-800 dark:text-slate-200">${stats.providerCostUSD.toFixed(2)}</strong>
            </div>
            <div className="flex justify-between pt-1 text-sm">
              <span className="font-semibold text-slate-700">Total</span>
              <strong className="text-slate-900 dark:text-white font-bold">
                ${(stats.platformCostUSD + stats.providerCostUSD).toFixed(2)}
              </strong>
            </div>
          </div>
          <div className="p-3 rounded-lg bg-slate-50 dark:bg-slate-800/50 text-xs text-slate-500">
            * Os valores são estimativas aproximadas baseadas nos tokens de entrada/saída de áudio consumidos pelos modelos de tempo real.
          </div>
        </div>

        {/* Status de Canais/Linhas */}
        <div className="bg-white dark:bg-slate-900 border rounded-xl p-5 space-y-4 col-span-2">
          <h3 className="font-semibold text-slate-800 dark:text-white flex items-center gap-2">
            <BarChart3 className="h-4 w-4 text-slate-500" /> Ocupação de Linhas no Pool
          </h3>
          <div className="space-y-3">
            {instances.map((inst, idx) => (
              <div key={idx} className="flex justify-between items-center p-3 border rounded-lg">
                <div>
                  <h4 className="text-sm font-semibold text-slate-800 dark:text-white">{inst.name}</h4>
                  <span className="text-xs text-slate-400">Status: {inst.status}</span>
                </div>
                <div className="text-right">
                  <span className="block text-xs font-semibold">{inst.active} / {inst.max} chamadas</span>
                  <div className="w-24 bg-slate-100 dark:bg-slate-800 h-2 rounded-full overflow-hidden mt-1">
                    <div 
                      className={`h-full ${inst.active === inst.max ? "bg-amber-500" : "bg-emerald-500"}`} 
                      style={{ width: `${(inst.active / inst.max) * 100}%` }}
                    />
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
};
