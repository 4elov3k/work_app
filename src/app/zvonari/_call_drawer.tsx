"use client"
// Панель разбора звонка — выезжает справа поверх списка звонков вместо
// прежней раскрывающейся строки (zvonari-ui-redesign v2, повторяет
// структуру прототипа htmlprew.html: плеер + статус-чипы одним блоком,
// подчёркнутые табы, сгруппированные по статусу шаги, транскрипт
// чат-бабблами с перемоткой по клику).
import { ReactNode, useEffect, useRef, useState } from "react";
import { Ban, ChevronLeft, ChevronRight, Download, Pause, Play, Sparkles, X } from "lucide-react";

import { Call, CallAnalytics, zvonariAPI } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { DIRECTION_LABELS, ERROR_KIND_LABELS, formatDuration, OutcomeBadge, STEP_KEYS, STEP_LABELS, stepStatusMeta } from "./_shared";

type DrawerTab = "steps" | "transcript";

export function CallDrawer({
  call,
  calls,
  initialTab,
  onClose,
  onNavigate,
  onAnalyze,
  isAnalyzing,
}: {
  call: Call | null;
  // Список, внутри которого работает "предыдущий/следующий" — обычно
  // отфильтрованный список звонков (не только текущая страница), чтобы
  // навигация не упиралась в границу страницы раньше, чем в границу списка.
  calls: Call[];
  initialTab: DrawerTab;
  onClose: () => void;
  onNavigate: (direction: 1 | -1) => void;
  onAnalyze: (callId: string) => void;
  isAnalyzing: boolean;
}) {
  const audioRef = useRef<HTMLAudioElement>(null);
  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [speed, setSpeed] = useState(1);
  const [tab, setTab] = useState<DrawerTab>(initialTab);

  const open = call !== null;
  const idx = call ? calls.findIndex((c) => c.id === call.id) : -1;

  // Каждый новый открытый звонок — чистый плеер и вкладка по умолчанию, а
  // не хвост состояния от предыдущего (иначе, например, позиция перемотки
  // предыдущего звонка мелькала бы на новом до onLoadedMetadata).
  useEffect(() => {
    setPlaying(false);
    setCurrentTime(0);
    setDuration(0);
    setSpeed(1);
    setTab(initialTab);
    // initialTab сознательно не в deps — это "начальное" значение только на
    // момент открытия конкретного звонка (call?.id), не то, что должно
    // дёргать вкладку обратно при каждом ре-рендере родителя.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [call?.id]);

  useEffect(() => {
    if (audioRef.current) audioRef.current.playbackRate = speed;
  }, [speed]);

  useEffect(() => {
    if (!open) return;
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
      if (e.key === "ArrowRight") onNavigate(1);
      if (e.key === "ArrowLeft") onNavigate(-1);
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open, onClose, onNavigate]);

  function seekTo(seconds: number) {
    const audio = audioRef.current;
    if (audio) audio.currentTime = seconds;
    setCurrentTime(seconds);
  }

  function togglePlay() {
    const audio = audioRef.current;
    if (!audio) return;
    if (playing) audio.pause();
    else audio.play().catch(() => {});
  }

  function cycleSpeed() {
    setSpeed((s) => (s === 1 ? 1.5 : s === 1.5 ? 2 : 1));
  }

  const analytics = call?.analytics_json;
  const isFraud = !!analytics?.fraud_suspected;
  const noRecording = call?.transcript_status === "no_recording";
  const needsAnalysis = !!call?.transcript_text && !analytics?.outcome;

  return (
    <>
      <div
        className={`fixed inset-0 z-40 bg-background/60 backdrop-blur-[1px] transition-opacity ${
          open ? "opacity-100" : "pointer-events-none opacity-0"
        }`}
        onClick={onClose}
        aria-hidden="true"
      />
      <aside
        className={`fixed right-0 top-0 z-50 flex h-full w-full max-w-[640px] flex-col border-l border-border bg-background shadow-2xl transition-transform duration-300 ${
          open ? "translate-x-0" : "translate-x-full"
        }`}
        role="dialog"
        aria-modal="true"
        aria-hidden={!open}
      >
        {call && (
          <>
            <div className="flex items-center gap-2 px-5 py-4 border-b border-border">
              <div className="min-w-0">
                <div className="truncate text-[15px] font-semibold text-foreground">
                  {new Date(call.started_at).toLocaleString("ru-RU")}
                </div>
                <div className="mt-0.5 text-xs text-muted-foreground">
                  Звонок {idx + 1} из {calls.length} · {formatDuration(call.duration_sec)} ·{" "}
                  {DIRECTION_LABELS[call.direction] || call.direction}
                </div>
              </div>
              <div className="ml-auto flex items-center gap-1">
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-8 w-8 p-0"
                      onClick={() => onNavigate(-1)}
                      disabled={idx <= 0}
                      aria-label="Предыдущий звонок"
                    >
                      <ChevronLeft className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Предыдущий (←)</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-8 w-8 p-0"
                      onClick={() => onNavigate(1)}
                      disabled={idx < 0 || idx >= calls.length - 1}
                      aria-label="Следующий звонок"
                    >
                      <ChevronRight className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Следующий (→)</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={onClose} aria-label="Закрыть">
                      <X className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Закрыть (Esc)</TooltipContent>
                </Tooltip>
              </div>
            </div>

            {/* Плеер + статус-чипы одним блоком (как в прототипе — без
                лишнего разделителя между ними) */}
            <div className="border-b border-border px-5 py-4">
              {noRecording ? (
                <p className="rounded-md border border-dashed border-border bg-muted/40 px-3 py-2.5 text-sm text-muted-foreground">
                  Для этого звонка нет записи (OnlinePBX подтвердил отсутствие).
                </p>
              ) : (
                <>
                  <audio
                    ref={audioRef}
                    src={zvonariAPI.recordingUrl(call.id)}
                    preload="none"
                    onPlay={() => setPlaying(true)}
                    onPause={() => setPlaying(false)}
                    onTimeUpdate={(e) => setCurrentTime(e.currentTarget.currentTime)}
                    onLoadedMetadata={(e) => setDuration(e.currentTarget.duration)}
                    onEnded={() => setPlaying(false)}
                  />
                  <div className="flex items-center gap-3 rounded-md border border-border bg-muted/40 px-3 py-2.5">
                    <Button
                      size="sm"
                      className="h-9 w-9 shrink-0 rounded-full p-0"
                      onClick={togglePlay}
                      aria-label={playing ? "Пауза" : "Слушать"}
                    >
                      {playing ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4 translate-x-0.5" />}
                    </Button>
                    <div
                      className="relative h-1.5 flex-1 cursor-pointer rounded-full bg-border"
                      onClick={(e) => {
                        const rect = e.currentTarget.getBoundingClientRect();
                        const fraction = (e.clientX - rect.left) / rect.width;
                        seekTo(fraction * (duration || 0));
                      }}
                    >
                      <div
                        className="absolute left-0 top-0 h-full rounded-full bg-primary"
                        style={{ width: duration ? `${Math.min(100, (currentTime / duration) * 100)}%` : "0%" }}
                      />
                    </div>
                    <span className="whitespace-nowrap font-mono text-xs tabular-nums text-muted-foreground">
                      {formatDuration(Math.floor(currentTime))} /{" "}
                      {duration ? formatDuration(Math.floor(duration)) : formatDuration(call.duration_sec)}
                    </span>
                    <Button variant="outline" size="sm" className="h-7 px-2 text-xs font-medium" onClick={cycleSpeed}>
                      {speed}×
                    </Button>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <a
                          href={zvonariAPI.recordingUrl(call.id)}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="shrink-0 text-muted-foreground transition-colors hover:text-foreground"
                          aria-label="Скачать запись"
                        >
                          <Download className="h-4 w-4" />
                        </a>
                      </TooltipTrigger>
                      <TooltipContent>Скачать запись</TooltipContent>
                    </Tooltip>
                  </div>
                </>
              )}

              <div className="mt-3 flex flex-wrap items-center gap-1.5">
                <OutcomeBadge outcome={analytics?.outcome} />
                {isFraud && (
                  <Badge variant="destructive" className="px-2 py-0.5 text-[11px] font-medium">
                    фрод
                  </Badge>
                )}
                {call.transcript_status !== "done" && (
                  <span className="rounded-full bg-muted px-2.5 py-1 text-xs text-muted-foreground">
                    {call.error_kind ? ERROR_KIND_LABELS[call.error_kind] || call.error_kind : call.transcript_status}
                  </span>
                )}
                {analytics?.counterpart_role && <MetaPill label="Собеседник" value={analytics.counterpart_role} />}
                {analytics?.lpr_confirmed && <MetaPill label="ЛПР подтверждён" value={analytics.lpr_confirmed} />}
                {typeof analytics?.max_step_reached === "number" && (
                  <MetaPill label="Макс. шаг" value={`${analytics.max_step_reached}/6`} />
                )}
                {analytics?.confidence && <MetaPill label="Уверенность" value={analytics.confidence} />}
                {needsAnalysis && (
                  <Button
                    size="sm"
                    className="ml-auto h-7 gap-1.5 bg-warning px-2.5 text-xs font-medium text-warning-foreground hover:bg-warning/90"
                    disabled={isAnalyzing}
                    onClick={() => onAnalyze(call.id)}
                  >
                    <Sparkles className="h-3.5 w-3.5" />
                    Разобрать звонок
                  </Button>
                )}
              </div>
            </div>

            <Tabs value={tab} onValueChange={(v) => setTab(v as DrawerTab)} className="flex min-h-0 flex-1 flex-col">
              <TabsList className="h-auto w-full justify-start gap-1 rounded-none border-b border-border bg-transparent p-0 px-5">
                <TabsTrigger
                  value="steps"
                  className="rounded-none border-b-2 border-transparent px-3 py-2 text-sm font-normal text-muted-foreground shadow-none data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:font-medium data-[state=active]:text-foreground data-[state=active]:shadow-none"
                >
                  Разбор по шагам
                </TabsTrigger>
                <TabsTrigger
                  value="transcript"
                  className="rounded-none border-b-2 border-transparent px-3 py-2 text-sm font-normal text-muted-foreground shadow-none data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:font-medium data-[state=active]:text-foreground data-[state=active]:shadow-none"
                >
                  Транскрипт
                </TabsTrigger>
              </TabsList>
              <div className="flex-1 overflow-y-auto px-5 py-4">
                <TabsContent value="steps" className="mt-0">
                  <DrawerSteps analytics={analytics} hangupCause={call.hangup_cause} hasTranscript={!!call.transcript_text} />
                </TabsContent>
                <TabsContent value="transcript" className="mt-0">
                  <TranscriptView call={call} currentTime={currentTime} onSeek={seekTo} />
                </TabsContent>
              </div>
            </Tabs>
          </>
        )}
      </aside>
    </>
  );
}

function MetaPill({ label, value }: { label: string; value: string }) {
  return (
    <span className="rounded-full bg-muted px-2.5 py-1 text-xs text-muted-foreground">
      {label}: <span className="font-medium text-foreground">{value}</span>
    </span>
  );
}

// Разбор по шагам — сначала срывы (красные карточки с доказательством), потом
// пройденные шаги (список с чекмарками), потом свёрнутый список
// неприменимых/недостигнутых, и в конце место срыва + рекомендация. Тот же
// порядок чтения, что в прототипе (htmlprew.html, stepsBlock) — самое важное
// (что сломалось) видно сразу, не после скролла мимо пройденных шагов.
const stepNumber = (key: string) => key.replace("step", "");

function DrawerSteps({
  analytics,
  hangupCause,
  hasTranscript,
}: {
  analytics?: CallAnalytics;
  hangupCause?: string;
  hasTranscript: boolean;
}) {
  const [showRest, setShowRest] = useState(false);

  if (!analytics?.steps) {
    return (
      <div className="space-y-2">
        {hangupCause && <p className="text-xs text-muted-foreground">Причина завершения: {hangupCause}</p>}
        <p className="rounded-md border border-dashed border-border bg-muted/40 p-3 text-sm text-muted-foreground">
          {hasTranscript
            ? "Этот звонок ещё не разобран. Транскрипт доступен — запустите анализ, чтобы получить разбор по шагам."
            : "Транскрипт ещё не готов."}
        </p>
      </div>
    );
  }

  const steps = analytics.steps;
  const entries = STEP_KEYS.map((key) => ({ key, step: steps[key] })).filter(
    (e): e is { key: (typeof STEP_KEYS)[number]; step: NonNullable<typeof e.step> } => !!e.step
  );
  const failed = entries.filter((e) => e.step.status === "Не выполнен" || e.step.status === "Не использован");
  const rest = entries.filter(
    (e) =>
      !failed.includes(e) &&
      (e.step.status === "Не применим" || e.step.status === "Не оценивается / недостаточно данных" || !e.step.status)
  );
  const done = entries.filter((e) => !failed.includes(e) && !rest.includes(e));

  return (
    <div className="space-y-3">
      {failed.map(({ key, step }) => (
        <div key={key} className="rounded-md border border-destructive/30 bg-destructive/5 p-3">
          <div className="flex flex-wrap items-center gap-2 text-sm font-medium text-destructive">
            {stepStatusMeta(step.status).icon}
            {STEP_LABELS[key]}
            <Badge variant="outline" className="ml-auto border-destructive/40 text-[11px] font-medium text-destructive">
              {step.status}
            </Badge>
          </div>
          {"evidence" in step && step.evidence && (
            <p className="mt-1.5 text-[13px] text-muted-foreground">
              <span className="text-muted-foreground/70">Доказательство:</span> {step.evidence}
            </p>
          )}
          {"missing" in step && step.missing && (
            <p className="mt-1 text-[13px] text-muted-foreground">
              <span className="text-muted-foreground/70">Чего не хватило:</span> {step.missing}
            </p>
          )}
        </div>
      ))}

      {done.length > 0 && (
        <div className="divide-y divide-border/60">
          {done.map(({ key, step }) => {
            const meta = stepStatusMeta(step.status);
            return (
              <div key={key} className="flex items-start gap-2.5 py-2 first:pt-0">
                <span className={`mt-0.5 shrink-0 ${meta.className}`}>{meta.icon}</span>
                <div className="min-w-0 text-sm">
                  <span className="font-medium text-foreground">{STEP_LABELS[key]}</span>{" "}
                  <span className={`text-[11px] ${meta.className}`}>{step.status}</span>
                  {"evidence" in step && step.evidence && (
                    <p className="mt-0.5 text-[13px] text-muted-foreground">{step.evidence}</p>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {rest.length > 0 && (
        <div>
          <button
            type="button"
            className="flex items-center gap-1.5 py-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
            onClick={() => setShowRest((v) => !v)}
          >
            <ChevronRight className={`h-3.5 w-3.5 transition-transform ${showRest ? "rotate-90" : ""}`} />
            Шаги {stepNumber(rest[0].key)}–6 · не применимы ({rest.length})
          </button>
          {showRest && (
            <div className="divide-y divide-border/40">
              {rest.map(({ key, step }) => (
                <div key={key} className="flex items-center gap-2.5 py-2 text-sm text-muted-foreground">
                  <Ban className="h-3.5 w-3.5 shrink-0" />
                  <span>{STEP_LABELS[key]}</span>
                  <span className="ml-auto text-[11px]">{step.status || "нет данных"}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {(analytics.break_point || analytics.recommendation) && (
        <div className="space-y-1.5 border-t border-border/70 pt-3 text-[13px]">
          {analytics.break_point && (
            <p className="text-muted-foreground">
              <span className="text-muted-foreground/70">Место срыва:</span> {analytics.break_point}
            </p>
          )}
          {analytics.recommendation && (
            <p className="rounded-md border-l-2 border-primary bg-primary/5 px-3 py-2 text-foreground">
              <span className="font-medium">Рекомендация:</span> {analytics.recommendation}
            </p>
          )}
        </div>
      )}
    </div>
  );
}

// Транскрипт — чат-бабблы с таймкодом и кликом-перемоткой (htmlprew.html,
// trBlock), работает только для звонков с transcript_segments (расшифрованы
// после введения этого поля — см. миграцию 034 в backend/migrations и
// transcribe_server.py в hermes). Более старые звонки показывают обычный
// плоский текст без перемотки — бэкфилла нет (решение с пользователем,
// старые звонки перетранскрибируются позже вместе с редизайном промпта
// фрод-детекции).
function TranscriptView({
  call,
  currentTime,
  onSeek,
}: {
  call: Call;
  currentTime: number;
  onSeek: (seconds: number) => void;
}) {
  const segments = call.transcript_segments;
  const proofText = call.analytics_json?.break_point;

  if (segments && segments.length > 0) {
    return (
      <div>
        {segments.map((seg, i) => {
          const active = currentTime >= seg.start && currentTime < seg.end;
          let body: ReactNode = seg.text;
          if (proofText) {
            const at = seg.text.indexOf(proofText);
            if (at >= 0) {
              body = (
                <>
                  {seg.text.slice(0, at)}
                  <mark className="rounded bg-warning/30 px-0.5 text-foreground">{proofText}</mark>
                  {seg.text.slice(at + proofText.length)}
                </>
              );
            }
          }
          return (
            <button
              key={i}
              type="button"
              onClick={() => onSeek(seg.start)}
              title={`Перейти к ${formatDuration(Math.floor(seg.start))}`}
              className={`group flex w-full items-start gap-3 border-b border-border/40 py-2.5 text-left transition-colors last:border-0 hover:bg-accent/40 ${
                active ? "bg-accent/60" : ""
              }`}
            >
              <span className="w-9 shrink-0 pt-1 font-mono text-[11px] tabular-nums text-muted-foreground">
                {formatDuration(Math.floor(seg.start))}
              </span>
              <span
                className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-[11px] font-medium ${
                  seg.speaker === 2 ? "bg-muted text-muted-foreground" : "bg-primary/15 text-primary"
                }`}
              >
                {seg.speaker ?? "?"}
              </span>
              <span className="min-w-0 flex-1 pt-0.5 text-[13px] leading-relaxed text-foreground/90 group-hover:text-foreground">
                {seg.speaker != null && <span className="font-medium">Собеседник {seg.speaker}: </span>}
                {body}
              </span>
            </button>
          );
        })}
      </div>
    );
  }
  if (!call.transcript_text) {
    return <p className="text-sm text-muted-foreground">Транскрипт недоступен.</p>;
  }
  return (
    <div className="space-y-1.5">
      <p className="whitespace-pre-wrap rounded-md bg-muted/50 p-3 text-sm text-muted-foreground">{call.transcript_text}</p>
      <p className="text-xs text-muted-foreground">
        Таймкоды по фразам недоступны — звонок расшифрован до появления этой функции.
      </p>
    </div>
  );
}
