"use client"
import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowLeft, RefreshCw, FileBarChart, Phone } from "lucide-react";

import { zvonariAPI, Caller, CallerReport, Call, ApiError } from "@/lib/api";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

function todayISO(offsetDays = 0): string {
  const d = new Date();
  d.setDate(d.getDate() + offsetDays);
  return d.toISOString().slice(0, 10);
}

function DistributionBars({ distribution }: { distribution: Record<string, number> }) {
  const entries = Object.entries(distribution);
  const max = Math.max(1, ...entries.map(([, count]) => count));
  if (entries.length === 0) {
    return <p className="text-sm text-muted-foreground">Нет звонков за этот период</p>;
  }
  return (
    <div className="space-y-2">
      {entries.map(([outcome, count]) => (
        <div key={outcome} className="flex items-center gap-3 text-sm">
          <span className="w-40 shrink-0 truncate text-muted-foreground">{outcome}</span>
          <div className="h-3 flex-1 rounded bg-muted overflow-hidden">
            <div
              className="h-full bg-primary"
              style={{ width: `${(count / max) * 100}%` }}
            />
          </div>
          <span className="w-6 shrink-0 text-right font-medium">{count}</span>
        </div>
      ))}
    </div>
  );
}

const DIRECTION_LABELS: Record<string, string> = {
  outbound: "исходящий",
  inbound: "входящий",
  local: "внутренний",
};

function formatDuration(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${String(s).padStart(2, "0")}`;
}

function CallDetailList({ calls }: { calls: Call[] }) {
  if (calls.length === 0) {
    return <p className="text-sm text-muted-foreground">Нет звонков за этот период</p>;
  }
  return (
    <div className="space-y-2">
      {calls.map((call) => (
        <details key={call.id} className="rounded border p-2">
          <summary className="flex cursor-pointer flex-wrap items-center gap-2 text-sm">
            <span className="text-muted-foreground">
              {new Date(call.started_at).toLocaleString("ru-RU")}
            </span>
            <Badge variant="outline">{DIRECTION_LABELS[call.direction] || call.direction}</Badge>
            <span className="text-muted-foreground">{formatDuration(call.duration_sec)}</span>
            {call.analytics_json?.outcome && <Badge variant="secondary">{call.analytics_json.outcome}</Badge>}
            {call.transcript_status !== "done" && (
              <span className="text-xs text-muted-foreground">({call.transcript_status})</span>
            )}
          </summary>
          <p className="mt-2 whitespace-pre-wrap text-sm text-muted-foreground">
            {call.transcript_text || "Транскрипт недоступен"}
          </p>
        </details>
      ))}
    </div>
  );
}

interface CallerPanelState {
  loading: boolean;
  error: string;
  distribution: Record<string, number> | null;
  report: CallerReport | null;
  calls: Call[] | null;
}

export default function ZvonariPage() {
  const [callers, setCallers] = useState<Caller[]>([]);
  const [loadingCallers, setLoadingCallers] = useState(true);
  const [listError, setListError] = useState("");

  const [from, setFrom] = useState(todayISO(-6));
  const [to, setTo] = useState(todayISO());

  const [syncing, setSyncing] = useState(false);
  const [syncError, setSyncError] = useState("");
  const [syncMessage, setSyncMessage] = useState("");

  const [panels, setPanels] = useState<Record<string, CallerPanelState>>({});
  const [callCounts, setCallCounts] = useState<Record<string, number>>({});

  const loadCallers = () => {
    setLoadingCallers(true);
    zvonariAPI
      .getCallers()
      .then((response) => {
        setCallers(response.data || []);
        setListError("");
      })
      .catch((err) => {
        console.error("Failed to load callers:", err);
        setListError("Не удалось загрузить список звонарей.");
      })
      .finally(() => setLoadingCallers(false));
  };

  const loadCallCounts = (periodFrom: string, periodTo: string) => {
    zvonariAPI
      .getCallCounts(periodFrom, periodTo)
      .then((response) => setCallCounts(response.data || {}))
      .catch((err) => console.error("Failed to load call counts:", err));
  };

  // Синхронизация идёт в фоне на бэкенде (может занимать минуты — сотни
  // звонков, каждый со своей транскрибацией и анализом), поэтому опрашиваем
  // статус вместо того, чтобы держать один долгий запрос — иначе прокси/
  // браузер обрывает соединение раньше, чем бэкенд успевает закончить.
  const pollSyncStatus = () => {
    const interval = setInterval(async () => {
      try {
        const response = await zvonariAPI.getSyncStatus();
        const status = response.data;
        if (!status.running) {
          clearInterval(interval);
          setSyncing(false);
          if (status.error) {
            setSyncError(status.error);
          } else if (status.result) {
            const r = status.result;
            setSyncMessage(
              `Найдено звонков: ${r.calls_found}, новых: ${r.calls_new}, пропущено: ${r.calls_skipped}` +
                (r.transcribe_errors > 0 ? `, ошибок транскрибации: ${r.transcribe_errors}` : "")
            );
          }
          loadCallers();
          loadCallCounts(from, to);
        }
      } catch (err) {
        console.error("Sync status poll failed:", err);
        clearInterval(interval);
        setSyncing(false);
      }
    }, 3000);
  };

  // При открытии страницы проверяем, не идёт ли уже синхронизация (например,
  // запущенная ранее и не завершившаяся к моменту перезагрузки страницы).
  useEffect(() => {
    loadCallers();
    zvonariAPI
      .getSyncStatus()
      .then((response) => {
        if (response.data.running) {
          setSyncing(true);
          setSyncMessage("Синхронизация уже выполняется...");
          pollSyncStatus();
        }
      })
      .catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Счётчик звонков на карточках всегда отражает текущий выбранный период.
  useEffect(() => {
    loadCallCounts(from, to);
  }, [from, to]);

  const period = useMemo(() => ({ from, to }), [from, to]);

  const handleSync = async () => {
    setSyncing(true);
    setSyncError("");
    setSyncMessage("Синхронизация запущена, это может занять несколько минут...");
    try {
      await zvonariAPI.sync(period.from, period.to);
      pollSyncStatus();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setSyncMessage("Синхронизация уже выполняется, ожидаем завершения...");
        pollSyncStatus();
        return;
      }
      console.error("Sync failed:", err);
      setSyncError(err instanceof Error ? err.message : "Не удалось синхронизировать звонки");
      setSyncing(false);
    }
  };

  const handleRequestReport = async (callerId: string) => {
    setPanels((current) => ({
      ...current,
      [callerId]: {
        loading: true,
        error: "",
        distribution: current[callerId]?.distribution ?? null,
        report: null,
        calls: current[callerId]?.calls ?? null,
      },
    }));
    try {
      const [distResponse, reportResponse, callsResponse] = await Promise.all([
        zvonariAPI.getDistribution(callerId, period.from, period.to),
        zvonariAPI.requestReport(callerId, "custom", period.from, period.to),
        zvonariAPI.getCalls(callerId, period.from, period.to),
      ]);
      setPanels((current) => ({
        ...current,
        [callerId]: {
          loading: false,
          error: "",
          distribution: distResponse.data,
          report: reportResponse.data,
          calls: callsResponse.data,
        },
      }));
    } catch (err) {
      console.error("Report request failed:", err);
      setPanels((current) => ({
        ...current,
        [callerId]: {
          loading: false,
          error: err instanceof Error ? err.message : "Не удалось получить отчёт",
          distribution: current[callerId]?.distribution ?? null,
          report: null,
          calls: current[callerId]?.calls ?? null,
        },
      }));
    }
  };

  return (
    <div className="container mx-auto py-8 px-4 max-w-5xl">
      <div className="mb-8 flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div>
          <div className="mb-2">
            <Link href="/">
              <Button variant="outline">
                <ArrowLeft className="mr-2 h-4 w-4" />
                Главная
              </Button>
            </Link>
          </div>
          <h1 className="text-4xl font-bold tracking-tight mb-2">Звонари</h1>
          <p className="text-muted-foreground">
            Звонки из OnlinePBX, транскрибация (Whisper локально) и аналитика по запросу через Hermes
          </p>
        </div>
      </div>

      <Card className="mb-8">
        <CardHeader>
          <CardTitle className="text-lg">Синхронизация звонков</CardTitle>
          <CardDescription>Период для загрузки CDR и записей разговоров из OnlinePBX</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap items-end gap-3">
            <div>
              <label className="mb-1 block text-sm text-muted-foreground">С</label>
              <Input type="date" value={from} onChange={(e) => setFrom(e.target.value)} className="w-40" />
            </div>
            <div>
              <label className="mb-1 block text-sm text-muted-foreground">По</label>
              <Input type="date" value={to} onChange={(e) => setTo(e.target.value)} className="w-40" />
            </div>
            <Button onClick={handleSync} disabled={syncing}>
              <RefreshCw className={`mr-2 h-4 w-4 ${syncing ? "animate-spin" : ""}`} />
              {syncing ? "Синхронизация..." : "Синхронизировать"}
            </Button>
          </div>
          {syncError && <p className="mt-3 text-sm text-destructive">{syncError}</p>}
          {syncMessage && <p className="mt-3 text-sm text-muted-foreground">{syncMessage}</p>}
        </CardContent>
      </Card>

      {listError && <p className="mb-4 text-sm text-destructive">{listError}</p>}
      {loadingCallers ? (
        <p className="text-muted-foreground">Загрузка...</p>
      ) : callers.length === 0 ? (
        <p className="text-muted-foreground">
          Звонарей пока нет — нажмите «Синхронизировать», чтобы подтянуть список из OnlinePBX.
        </p>
      ) : (
        <div className="space-y-4">
          {callers.map((caller) => {
            const panel = panels[caller.id];
            return (
              <Card key={caller.id}>
                <CardHeader className="flex flex-row items-center justify-between space-y-0">
                  <div>
                    <CardTitle className="text-base">{caller.name}</CardTitle>
                    <CardDescription className="flex items-center gap-2">
                      <span>внутр. номер {caller.pbx_extension}</span>
                      <span className="inline-flex items-center gap-1">
                        <Phone className="h-3 w-3" />
                        {callCounts[caller.id] ?? 0} звонков за период
                      </span>
                    </CardDescription>
                  </div>
                  <div className="flex items-center gap-2">
                    {!caller.active && <Badge variant="secondary">неактивен</Badge>}
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleRequestReport(caller.id)}
                      disabled={panel?.loading}
                    >
                      <FileBarChart className="mr-2 h-4 w-4" />
                      {panel?.loading ? "Формирование..." : "Запросить отчёт"}
                    </Button>
                  </div>
                </CardHeader>
                {panel && (panel.error || panel.distribution || panel.report || panel.calls) && (
                  <CardContent className="space-y-4">
                    {panel.error && <p className="text-sm text-destructive">{panel.error}</p>}
                    {panel.distribution && (
                      <div>
                        <h4 className="mb-2 text-sm font-medium">Распределение звонков за период</h4>
                        <DistributionBars distribution={panel.distribution} />
                      </div>
                    )}
                    {panel.report && (
                      <div>
                        <h4 className="mb-2 text-sm font-medium">Анализ за период</h4>
                        <p className="whitespace-pre-wrap text-sm text-muted-foreground">{panel.report.summary_text}</p>
                      </div>
                    )}
                    {panel.calls && (
                      <div>
                        <h4 className="mb-2 text-sm font-medium">Детализация по звонкам ({panel.calls.length})</h4>
                        <CallDetailList calls={panel.calls} />
                      </div>
                    )}
                  </CardContent>
                )}
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
