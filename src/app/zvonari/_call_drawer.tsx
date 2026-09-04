"use client"
// Панель разбора звонка — выезжает справа поверх списка звонков вместо
// прежней раскрывающейся строки (zvonari-ui-redesign v2, htmlprew.html).
// Даёт место для реального плеера записи и клика по фразе транскрипта →
// перемотка (Call.transcript_segments, задача "фундамент под drawer").
import { useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, Loader2, Mic, Pause, Play, Sparkles, X } from "lucide-react";

import { Call, zvonariAPI } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { DIRECTION_LABELS, ERROR_KIND_LABELS, formatDuration, OutcomeBadge, StepBreakdown } from "./_shared";

type DrawerTab = "steps" | "transcript";

export function CallDrawer({
  call,
  calls,
  initialTab,
  onClose,
  onNavigate,
  onRetranscribe,
  onAnalyze,
  isRetranscribing,
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
  onRetranscribe: (callId: string) => void;
  onAnalyze: (callId: string) => void;
  isRetranscribing: boolean;
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
            <div className="flex items-center gap-2 border-b border-border px-5 py-4">
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

            {/* Плеер — реальная запись через /zvonari/calls/{id}/recording
                (бэкенд резолвит свежую подписанную ссылку OnlinePBX и
                редиректит на неё при каждом обращении к <audio src>). */}
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
                      {formatDuration(Math.floor(currentTime))} / {duration ? formatDuration(Math.floor(duration)) : formatDuration(call.duration_sec)}
                    </span>
                    <Button variant="outline" size="sm" className="h-7 px-2 text-xs font-medium" onClick={cycleSpeed}>
                      {speed}×
                    </Button>
                  </div>
                </>
              )}
            </div>

            {/* Статус + действия */}
            <div className="flex flex-wrap items-center gap-2 border-b border-border px-5 py-3">
              <OutcomeBadge outcome={analytics?.outcome} />
              {isFraud && (
                <Badge variant="destructive" className="gap-1 px-1.5 py-0 text-[10px]">
                  фрод
                </Badge>
              )}
              {call.transcript_status !== "done" && (
                <Badge variant="outline" className="text-xs font-normal text-muted-foreground">
                  {call.error_kind ? ERROR_KIND_LABELS[call.error_kind] || call.error_kind : call.transcript_status}
                </Badge>
              )}
              <div className="ml-auto flex items-center gap-1">
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-7 px-2 text-muted-foreground hover:text-foreground"
                      disabled={isRetranscribing}
                      aria-label="Перетранскрибировать"
                      onClick={() => onRetranscribe(call.id)}
                    >
                      {isRetranscribing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Mic className="h-3.5 w-3.5" />}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Перетранскрибировать</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-7 px-2 text-muted-foreground hover:text-foreground"
                      disabled={isAnalyzing || !call.transcript_text}
                      aria-label="Переанализировать"
                      onClick={() => onAnalyze(call.id)}
                    >
                      {isAnalyzing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Sparkles className="h-3.5 w-3.5" />}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Переанализировать</TooltipContent>
                </Tooltip>
              </div>
            </div>

            <Tabs value={tab} onValueChange={(v) => setTab(v as DrawerTab)} className="flex min-h-0 flex-1 flex-col">
              <TabsList className="mx-5 mt-3 w-fit">
                <TabsTrigger value="steps">Разбор по шагам</TabsTrigger>
                <TabsTrigger value="transcript">Транскрипт</TabsTrigger>
              </TabsList>
              <div className="flex-1 overflow-y-auto px-5 py-4">
                <TabsContent value="steps" className="mt-0">
                  {analytics?.steps ? (
                    <StepBreakdown analytics={analytics} />
                  ) : (
                    <p className="rounded-md border border-dashed border-border bg-muted/40 p-3 text-sm text-muted-foreground">
                      {call.transcript_text
                        ? "Этот звонок ещё не разобран. Транскрипт доступен — запустите анализ, чтобы получить разбор по шагам."
                        : "Транскрипт ещё не готов."}
                    </p>
                  )}
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

// Клик по фразе перематывает плеер на её таймкод — работает только для
// звонков с transcript_segments (расшифрованных после введения этого поля,
// см. миграцию 034 в backend/migrations и hermes' transcribe_server.py).
// Более старые звонки показывают обычный плоский текст без перемотки —
// бэкфилла нет (решение с пользователем, старые звонки перетранскрибируются
// позже вместе с редизайном промпта фрод-детекции).
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
  if (segments && segments.length > 0) {
    return (
      <div className="space-y-0.5">
        {segments.map((seg, i) => {
          const active = currentTime >= seg.start && currentTime < seg.end;
          return (
            <button
              key={i}
              type="button"
              onClick={() => onSeek(seg.start)}
              title={`Перейти к ${formatDuration(Math.floor(seg.start))}`}
              className={`flex w-full gap-3 rounded-md px-2 py-2 text-left text-sm transition-colors hover:bg-accent/60 ${
                active ? "bg-accent" : ""
              }`}
            >
              <span className="w-10 shrink-0 pt-0.5 font-mono text-[11px] tabular-nums text-muted-foreground">
                {formatDuration(Math.floor(seg.start))}
              </span>
              <span className="min-w-0 flex-1 leading-relaxed text-foreground">
                {seg.speaker != null && <span className="font-medium">Собеседник {seg.speaker}: </span>}
                {seg.text}
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
    <div className="space-y-1">
      <p className="whitespace-pre-wrap rounded-md bg-muted/50 p-3 text-sm text-muted-foreground">{call.transcript_text}</p>
      <p className="text-xs text-muted-foreground">
        Таймкоды по фразам недоступны — звонок расшифрован до появления этой функции.
      </p>
    </div>
  );
}
