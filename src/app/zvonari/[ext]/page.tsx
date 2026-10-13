"use client"
import { useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft, Download, FileBarChart, History, Loader2, Sparkles } from "lucide-react";

import { zvonariAPI, Caller, CallerReport, Call } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { TooltipProvider } from "@/components/ui/tooltip";
import {
  loadStoredPeriod,
  PERIOD_STORAGE_KEY,
  todayISO,
  OUTCOME_SCRIPT_COMPLETED,
  PeriodControl,
  ScriptTrack,
  trackSegmentsFromDistribution,
  ScriptFunnel,
  CallDetailList,
} from "../_shared";

type Tab = "overview" | "calls" | "reports";
const TAB_VALUES: Tab[] = ["overview", "calls", "reports"];

export default function ZvonariCallerPage() {
  const params = useParams<{ ext: string }>();
  const ext = params.ext;

  const [callers, setCallers] = useState<Caller[] | null>(null);
  const [callersError, setCallersError] = useState("");
  useEffect(() => {
    zvonariAPI
      .getCallers()
      .then((response) => setCallers(response.data || []))
      .catch((err) => {
        console.error("Failed to load callers:", err);
        setCallersError("Не удалось загрузить список звонарей.");
      });
  }, []);

  const caller = useMemo(() => callers?.find((c) => c.pbx_extension === ext) ?? null, [callers, ext]);

  const [from, setFrom] = useState(todayISO(-6));
  const [to, setTo] = useState(todayISO());
  const [tab, setTab] = useState<Tab>("overview");
  const [roadmapFilter, setRoadmapFilter] = useState("");
  const [errorKindFilter, setErrorKindFilter] = useState("");
  const [callSearch, setCallSearch] = useState("");
  const [callPage, setCallPage] = useState(0);

  // Период приходит по ссылке из /zvonari (?from=&to=) — "период при переходе
  // сохраняется" (zvonari-ui-redesign.md §2). Если ссылки нет (открыли
  // страницу напрямую) — тот же localStorage, что и на списке. Восстанавливаем
  // после монтирования, чтобы не разойтись с серверным рендером при гидрации.
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const urlFrom = params.get("from");
    const urlTo = params.get("to");
    if (urlFrom && urlTo) {
      setFrom(urlFrom);
      setTo(urlTo);
    } else {
      const stored = loadStoredPeriod();
      if (stored) {
        setFrom(stored.from);
        setTo(stored.to);
      }
    }
    const urlTab = params.get("tab");
    if (urlTab && (TAB_VALUES as string[]).includes(urlTab)) setTab(urlTab as Tab);
    const urlOutcome = params.get("outcome");
    if (urlOutcome) setRoadmapFilter(urlOutcome);
    const urlError = params.get("error");
    if (urlError) setErrorKindFilter(urlError);
    const urlQ = params.get("q");
    if (urlQ) setCallSearch(urlQ);
    const urlPage = params.get("page");
    const parsedPage = urlPage ? parseInt(urlPage, 10) : NaN;
    if (!isNaN(parsedPage) && parsedPage >= 0) setCallPage(parsedPage);
  }, []);

  // Первый запуск при монтировании всегда видит ещё дефолтные from/to —
  // эффект восстановления выше в этом же проходе только планирует setState,
  // применится он лишь на следующем рендере. Без пропуска первого запуска
  // мы бы на долю секунды затирали URL дефолтом поверх того, что пришло по
  // ссылке или из localStorage (та же гонка, что чинили в /zvonari, см.
  // page.tsx). Заодно пишем период в тот же localStorage, что и список —
  // раньше период, изменённый прямо на карточке звонаря, никуда не
  // сохранялся и терялся при следующем заходе не по прямой ссылке.
  const skipUrlSync = useRef(true);
  useEffect(() => {
    if (skipUrlSync.current) {
      skipUrlSync.current = false;
      return;
    }
    const params = new URLSearchParams();
    params.set("from", from);
    params.set("to", to);
    if (tab !== "overview") params.set("tab", tab);
    if (roadmapFilter) params.set("outcome", roadmapFilter);
    if (errorKindFilter) params.set("error", errorKindFilter);
    if (callSearch) params.set("q", callSearch);
    if (callPage > 0) params.set("page", String(callPage));
    window.history.replaceState(null, "", `${window.location.pathname}?${params.toString()}`);
    window.localStorage.setItem(PERIOD_STORAGE_KEY, JSON.stringify({ from, to }));
  }, [from, to, tab, roadmapFilter, errorKindFilter, callSearch, callPage]);

  // Звонки периода — единственный источник данных для шапки, воронки и
  // таба "Звонки" разом (вместо трёх разных агрегатных эндпоинтов, как было
  // раньше): total/статусы/исходы/фрод всегда согласованы друг с другом,
  // потому что считаются из одного и того же массива. Загружается сразу
  // (не по клику "Показать звонки") — страница уже открыта на конкретного
  // звонаря, скрывать его звонки за лишним кликом незачем.
  const [calls, setCalls] = useState<Call[] | null>(null);
  const [callsLoading, setCallsLoading] = useState(false);
  const [callsError, setCallsError] = useState("");

  useEffect(() => {
    if (!caller) return;
    setCallsLoading(true);
    setCallsError("");
    zvonariAPI
      .getCalls(caller.id, from, to)
      .then((response) => setCalls(response.data || []))
      .catch((err) => {
        console.error("Failed to load calls:", err);
        setCallsError(err instanceof Error ? err.message : "Не удалось загрузить звонки");
      })
      .finally(() => setCallsLoading(false));
    // Сбрасываем фильтры таба "Звонки" при смене периода — та же логика,
    // что раньше жила в эффекте на [from, to] в списке.
    setRoadmapFilter("");
    setErrorKindFilter("");
    setCallSearch("");
    setCallPage(0);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [caller?.id, from, to]);

  const stats = useMemo(() => {
    const list = calls ?? [];
    const total = list.length;
    const distribution: Record<string, number> = {};
    let fraudCount = 0;
    let doneCount = 0;
    let problemCount = 0;
    for (const call of list) {
      const outcome = call.analytics_json?.outcome || "не проанализировано";
      distribution[outcome] = (distribution[outcome] ?? 0) + 1;
      if (call.analytics_json?.fraud_suspected) fraudCount++;
      if (call.transcript_status === "done") doneCount++;
      if (call.transcript_status === "failed" || call.transcript_status === "no_recording") problemCount++;
    }
    const unanalyzedCount = distribution["не проанализировано"] ?? 0;
    const analyzedCount = total - unanalyzedCount;
    const scriptCompletedCount = distribution[OUTCOME_SCRIPT_COMPLETED] ?? 0;
    const problemPct = total > 0 ? Math.round((problemCount / total) * 100) : 0;
    return { total, distribution, fraudCount, doneCount, unanalyzedCount, analyzedCount, scriptCompletedCount, problemPct };
  }, [calls]);

  const [report, setReport] = useState<CallerReport | null>(null);
  const [reportLoading, setReportLoading] = useState(false);
  const [reportError, setReportError] = useState("");
  const [history, setHistory] = useState<CallerReport[] | null>(null);
  const [historyLoading, setHistoryLoading] = useState(false);

  useEffect(() => {
    if (!caller) return;
    setHistoryLoading(true);
    zvonariAPI
      .getReportHistory(caller.id)
      .then((response) => setHistory(response.data || []))
      .catch((err) => console.error("Failed to load report history:", err))
      .finally(() => setHistoryLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [caller?.id]);

  const handleRequestReport = async () => {
    if (!caller) return;
    setReportLoading(true);
    setReportError("");
    try {
      const response = await zvonariAPI.requestReport(caller.id, "custom", from, to);
      setReport(response.data);
      setHistory((current) => (current ? [response.data, ...current] : [response.data]));
    } catch (err) {
      console.error("Report request failed:", err);
      setReportError(err instanceof Error ? err.message : "Не удалось получить отчёт");
    } finally {
      setReportLoading(false);
    }
  };

  const handleDownloadCsv = async () => {
    if (!caller) return;
    const url = zvonariAPI.exportCallsCsvUrl(caller.id, from, to);
    try {
      const response = await fetch(url);
      if (!response.ok) {
        const text = await response.text().catch(() => "");
        throw new Error(text || `Ошибка HTTP ${response.status}`);
      }
      const disposition = response.headers.get("content-disposition");
      const match = disposition?.match(/filename\*=UTF-8''([^;]+)/i) || disposition?.match(/filename="?([^";]+)"?/i);
      const filename = match?.[1] ? decodeURIComponent(match[1]) : "calls.csv";
      const blob = await response.blob();
      const blobUrl = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = blobUrl;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(blobUrl);
    } catch (err) {
      console.error("CSV export failed:", err);
      setCallsError(err instanceof Error ? err.message : "Не удалось выгрузить CSV");
    }
  };

  const [retranscribingIds, setRetranscribingIds] = useState<Set<string>>(new Set());
  const [analyzingIds, setAnalyzingIds] = useState<Set<string>>(new Set());

  const handleRetranscribeCall = async (callId: string) => {
    setRetranscribingIds((current) => new Set(current).add(callId));
    setCallsError("");
    try {
      const response = await zvonariAPI.retranscribeCall(callId);
      setCalls((current) => (current ? current.map((c) => (c.id === callId ? response.data : c)) : current));
    } catch (err) {
      console.error("Retranscribe failed:", err);
      setCallsError(err instanceof Error ? err.message : "Не удалось перетранскрибировать звонок");
    } finally {
      setRetranscribingIds((current) => {
        const next = new Set(current);
        next.delete(callId);
        return next;
      });
    }
  };

  const handleAnalyzeCall = async (callId: string) => {
    setAnalyzingIds((current) => new Set(current).add(callId));
    setCallsError("");
    try {
      const response = await zvonariAPI.analyzeCall(callId);
      setCalls((current) => (current ? current.map((c) => (c.id === callId ? response.data : c)) : current));
    } catch (err) {
      console.error("Analyze failed:", err);
      setCallsError(err instanceof Error ? err.message : "Не удалось переанализировать звонок");
    } finally {
      setAnalyzingIds((current) => {
        const next = new Set(current);
        next.delete(callId);
        return next;
      });
    }
  };

  const [periodAnalyzeMessage, setPeriodAnalyzeMessage] = useState("");
  const handleAnalyzePeriod = async () => {
    setPeriodAnalyzeMessage("Запускаем...");
    try {
      await zvonariAPI.analyzeCalls(from, to);
      setPeriodAnalyzeMessage("Анализ запущен для всего периода — прогресс виден на странице «Звонари», обновите эту страницу через минуту.");
    } catch (err) {
      console.error("Failed to start period analysis:", err);
      setPeriodAnalyzeMessage(err instanceof Error ? err.message : "Не удалось запустить анализ");
    }
  };

  if (callersError) {
    return (
      <div className="zvonari-theme min-h-screen bg-background">
        <div className="container mx-auto max-w-4xl px-4 py-10">
          <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{callersError}</p>
        </div>
      </div>
    );
  }

  if (callers && !caller) {
    return (
      <div className="zvonari-theme min-h-screen bg-background">
        <div className="container mx-auto max-w-4xl px-4 py-10">
          <p className="mb-4 text-muted-foreground">Звонарь с внутренним номером «{ext}» не найден.</p>
          <Link href="/zvonari">
            <Button variant="outline" size="sm">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Назад к списку
            </Button>
          </Link>
        </div>
      </div>
    );
  }

  if (!caller) {
    return (
      <div className="zvonari-theme min-h-screen bg-background">
        <div className="container mx-auto max-w-4xl px-4 py-10 text-muted-foreground">
          <Loader2 className="mr-2 inline h-4 w-4 animate-spin" />
          Загрузка...
        </div>
      </div>
    );
  }

  const listHref = `/zvonari?from=${from}&to=${to}`;

  return (
    <TooltipProvider delayDuration={200}>
    <div className="zvonari-theme min-h-screen bg-background">
      {/* Шапка звонаря — липкая, метрики всегда подписаны (не голые числа) —
          zvonari-ui-redesign.md §2. */}
      <div className="sticky top-0 z-10 border-b border-border bg-background/95 backdrop-blur">
        <div className="container mx-auto max-w-6xl px-4 py-3">
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <Link href={listHref} className="flex items-center gap-1 text-muted-foreground hover:text-foreground">
              <ArrowLeft className="h-3.5 w-3.5" />
              Звонари
            </Link>
            <span className="text-muted-foreground">/</span>
            <span className="font-medium">{caller.name}</span>
            <span className="text-muted-foreground">· {caller.pbx_extension}</span>
            <div className="ml-auto">
              <PeriodControl from={from} to={to} onChange={(f, t) => { setFrom(f); setTo(t); }} showPresets={false} />
            </div>
          </div>

          <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
            <span className="font-mono tabular-nums">{stats.total} звонков</span>
            <span className="font-mono tabular-nums text-muted-foreground">{stats.analyzedCount} проанализировано</span>
            <span className="font-mono tabular-nums text-success">{stats.scriptCompletedCount} до шага 6</span>
            {stats.total > 0 && (
              <span className={`font-mono tabular-nums ${stats.problemPct > 0 ? "text-destructive" : "text-muted-foreground"}`}>
                {stats.problemPct}% ошибок
              </span>
            )}
            {callsLoading && <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" />}
            <div className="ml-auto flex items-center gap-2">
              <Button variant="outline" size="sm" onClick={handleRequestReport} disabled={reportLoading}>
                {reportLoading ? <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" /> : <FileBarChart className="mr-2 h-3.5 w-3.5" />}
                {reportLoading ? "Формирование..." : "Отчёт"}
              </Button>
              <Button variant="ghost" size="sm" onClick={handleDownloadCsv} title="Скачать CSV" aria-label="Скачать CSV">
                <Download className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        </div>
      </div>

      <div className="container mx-auto max-w-6xl px-4 py-5">
        {callsError && (
          <p className="mb-4 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{callsError}</p>
        )}
        {reportError && (
          <p className="mb-4 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{reportError}</p>
        )}

        <Tabs value={tab} onValueChange={(v) => setTab(v as Tab)}>
          <TabsList>
            <TabsTrigger value="overview">Обзор</TabsTrigger>
            <TabsTrigger value="calls">Звонки {stats.total > 0 && `${stats.total}`}</TabsTrigger>
            <TabsTrigger value="reports">Отчёты {history && history.length > 0 && `${history.length}`}</TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="mt-4">
            {!callsLoading && stats.total > 0 && stats.analyzedCount === 0 ? (
              // Пустое состояние: приглашение нажать, а не сетка нулей
              // (zvonari-ui-redesign.md §2).
              <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-border bg-card px-4 py-3">
                <p className="text-sm text-muted-foreground">
                  {stats.total} {stats.total === 1 ? "звонок ждёт" : "звонков ждут"} анализа
                </p>
                <div className="flex items-center gap-2">
                  {periodAnalyzeMessage && <span className="text-xs text-muted-foreground">{periodAnalyzeMessage}</span>}
                  <Button size="sm" onClick={handleAnalyzePeriod} disabled={!!periodAnalyzeMessage}>
                    <Sparkles className="mr-2 h-3.5 w-3.5" />
                    Проанализировать период
                  </Button>
                </div>
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                <Card className="rounded-lg border-border shadow-none">
                  <CardContent className="pt-5">
                    <h4 className="mb-3 text-sm font-medium">Путь по скрипту</h4>
                    <div className="mb-4">
                      <ScriptTrack size="md" useTooltip segments={trackSegmentsFromDistribution(stats.distribution)} />
                    </div>
                    {stats.total === 0 && !callsLoading ? (
                      <p className="text-sm text-muted-foreground">Нет звонков за этот период</p>
                    ) : (
                      <ScriptFunnel
                        distribution={stats.distribution}
                        fraudCount={stats.fraudCount}
                        selected={roadmapFilter}
                        onSelect={(value) => {
                          setRoadmapFilter(value);
                          setTab("calls");
                        }}
                      />
                    )}
                  </CardContent>
                </Card>
                <Card className="rounded-lg border-border shadow-none">
                  <CardContent className="pt-5">
                    <h4 className="mb-3 flex items-center gap-1.5 text-sm font-medium">
                      <FileBarChart className="h-4 w-4 text-primary" />
                      Последний отчёт
                    </h4>
                    {historyLoading ? (
                      <p className="flex items-center gap-2 text-sm text-muted-foreground">
                        <Loader2 className="h-4 w-4 animate-spin" />
                        Загрузка...
                      </p>
                    ) : (report ?? history?.[0]) ? (
                      <>
                        <p className="mb-2 text-xs text-muted-foreground">
                          {new Date((report ?? history![0]).requested_at).toLocaleDateString("ru-RU")}, период{" "}
                          {(report ?? history![0]).period_start} – {(report ?? history![0]).period_end}
                        </p>
                        <p className="whitespace-pre-wrap text-sm text-muted-foreground">
                          {(report ?? history![0]).summary_text}
                        </p>
                        {history && history.length > 1 && (
                          <button
                            type="button"
                            onClick={() => setTab("reports")}
                            className="mt-3 text-xs text-primary hover:underline"
                          >
                            Ещё {history.length - 1} {history.length - 1 === 1 ? "отчёт" : "отчёта"} →
                          </button>
                        )}
                      </>
                    ) : (
                      <p className="text-sm text-muted-foreground">
                        Отчётов ещё не было. Нажмите «Отчёт» в шапке, чтобы сформировать первый.
                      </p>
                    )}
                  </CardContent>
                </Card>
              </div>
            )}
          </TabsContent>

          <TabsContent value="calls" className="mt-4">
            {callsLoading && !calls ? (
              <div className="flex items-center gap-2 text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                Загрузка звонков...
              </div>
            ) : (
              <CallDetailList
                calls={calls ?? []}
                search={callSearch}
                onSearchChange={setCallSearch}
                page={callPage}
                onPageChange={setCallPage}
                retranscribingIds={retranscribingIds}
                onRetranscribe={handleRetranscribeCall}
                analyzingIds={analyzingIds}
                onAnalyze={handleAnalyzeCall}
                roadmapFilter={roadmapFilter}
                onClearRoadmapFilter={() => setRoadmapFilter("")}
                errorKindFilter={errorKindFilter}
                onClearErrorKindFilter={() => setErrorKindFilter("")}
              />
            )}
          </TabsContent>

          <TabsContent value="reports" className="mt-4 space-y-3">
            {report && (
              <Card className="rounded-lg border-border shadow-none">
                <CardContent className="pt-5">
                  <h4 className="mb-2 flex items-center gap-1.5 text-sm font-medium">
                    <FileBarChart className="h-4 w-4 text-primary" />
                    Текущий отчёт
                  </h4>
                  <p className="whitespace-pre-wrap text-sm text-muted-foreground">{report.summary_text}</p>
                </CardContent>
              </Card>
            )}
            {historyLoading ? (
              <p className="flex items-center gap-2 text-sm text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                Загрузка истории...
              </p>
            ) : !history || history.length === 0 ? (
              <p className="text-sm text-muted-foreground">Отчётов по этому звонарю ещё не было</p>
            ) : (
              <div className="space-y-2">
                {history.map((r) => (
                  <details key={r.id} className="group rounded-md border border-border p-3 transition-colors hover:bg-accent/40">
                    <summary className="flex cursor-pointer items-center gap-2 text-sm text-muted-foreground group-open:text-foreground">
                      <History className="h-3.5 w-3.5" />
                      {new Date(r.requested_at).toLocaleString("ru-RU")} — {r.period_start} — {r.period_end}
                    </summary>
                    <p className="mt-2 whitespace-pre-wrap text-sm text-muted-foreground">{r.summary_text}</p>
                  </details>
                ))}
              </div>
            )}
          </TabsContent>
        </Tabs>
      </div>
    </div>
    </TooltipProvider>
  );
}
