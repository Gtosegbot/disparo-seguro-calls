import { useState, useEffect } from "react";
import { PlusCircle, Phone, Shield, BarChart3, ListFilter, Sliders } from "lucide-react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "@/components/ui/sonner";
import { AppShell } from "@/components/layout/AppShell";
import { CallsPage } from "@/pages/CallsPage";
import { LinesPage } from "@/pages/LinesPage";
import { CampaignPage } from "@/pages/CampaignPage";
import { DashboardPage } from "@/pages/DashboardPage";
import { AdminProvidersPage } from "@/pages/AdminProvidersPage";
import { SessionPairing } from "@/components/domain/session/SessionPairing";
import { SessionHeader } from "@/components/domain/session/SessionHeader";
import { IncomingCallModal } from "@/components/domain/call/IncomingCallModal";
import { TransferOfferModal } from "@/components/domain/call/TransferOfferModal";
import { EmptyState } from "@/components/shared/EmptyState";
import { ensureSessionsWired, useSessions } from "@/stores/sessions";
import { ensureCallsWired } from "@/stores/calls";
import { useTheme } from "@/stores/theme";

export const App = () => {
  const sessions = useSessions((s) => s.sessions);
  const activeId = useSessions((s) => s.activeId);
  const theme = useTheme((s) => s.theme);
  const [activeTab, setActiveTab] = useState("voip"); // voip, lines, campaigns, dashboard, admin_providers

  useEffect(() => {
    ensureSessionsWired();
    ensureCallsWired();
  }, []);

  const active = sessions.find((s) => s.id === activeId) ?? null;

  let mainContent;
  if (activeTab === "lines") {
    mainContent = <LinesPage />;
  } else if (activeTab === "campaigns") {
    mainContent = <CampaignPage />;
  } else if (activeTab === "dashboard") {
    mainContent = <DashboardPage />;
  } else if (activeTab === "admin_providers") {
    mainContent = <AdminProvidersPage />;
  } else {
    mainContent = active ? (
      active.paired ? <CallsPage sid={active.id} /> : <SessionPairing session={active} />
    ) : (
      <EmptyState title="Selecione uma conta" description="Escolha uma conta na barra lateral." />
    );
  }

  return (
    <TooltipProvider delayDuration={200}>
      <AppShell>
        {sessions.length === 0 ? (
          <EmptyState
            icon={<PlusCircle className="h-6 w-6" />}
            title="Nenhuma conta ainda"
            description="Crie sua primeira conta de WhatsApp na barra lateral para começar a ligar."
          />
        ) : (
          <div className="space-y-6">
            {active && <SessionHeader session={active} />}
            
            {/* Tabs de Navegação Comercial (White-Label) */}
            <div className="flex border-b border-slate-200 dark:border-slate-800 gap-4 text-sm font-semibold">
              <button
                onClick={() => setActiveTab("voip")}
                className={`flex items-center gap-1.5 pb-3 border-b-2 px-1 ${
                  activeTab === "voip" ? "border-slate-900 text-slate-900 dark:text-white dark:border-white" : "border-transparent text-slate-400"
                }`}
              >
                <Phone className="h-4 w-4" /> Telefonia VoIP
              </button>
              <button
                onClick={() => setActiveTab("lines")}
                className={`flex items-center gap-1.5 pb-3 border-b-2 px-1 ${
                  activeTab === "lines" ? "border-slate-900 text-slate-900 dark:text-white dark:border-white" : "border-transparent text-slate-400"
                }`}
              >
                <Shield className="h-4 w-4" /> Linhas
              </button>
              <button
                onClick={() => setActiveTab("campaigns")}
                className={`flex items-center gap-1.5 pb-3 border-b-2 px-1 ${
                  activeTab === "campaigns" ? "border-slate-900 text-slate-900 dark:text-white dark:border-white" : "border-transparent text-slate-400"
                }`}
              >
                <ListFilter className="h-4 w-4" /> Campanhas
              </button>
              <button
                onClick={() => setActiveTab("dashboard")}
                className={`flex items-center gap-1.5 pb-3 border-b-2 px-1 ${
                  activeTab === "dashboard" ? "border-slate-900 text-slate-900 dark:text-white dark:border-white" : "border-transparent text-slate-400"
                }`}
              >
                <BarChart3 className="h-4 w-4" /> Observabilidade
              </button>
              <button
                onClick={() => setActiveTab("admin_providers")}
                className={`flex items-center gap-1.5 pb-3 border-b-2 px-1 ${
                  activeTab === "admin_providers" ? "border-slate-900 text-slate-900 dark:text-white dark:border-white" : "border-transparent text-slate-400"
                }`}
              >
                <Sliders className="h-4 w-4" /> Admin Tiers
              </button>
            </div>

            {/* Conteúdo Ativo */}
            <div className="pt-2">
              {mainContent}
            </div>
          </div>
        )}
      </AppShell>
      <IncomingCallModal />
      <TransferOfferModal />
      <Toaster theme={theme} position="top-right" richColors closeButton />
    </TooltipProvider>
  );
};
