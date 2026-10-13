"use client"
import { useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { ArrowLeft, Download, Loader2, ShieldAlert, CheckCircle2, XCircle, Play, Pause } from "lucide-react";

import { zvonariAPI, Caller, Call } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { PeriodControl, todayISO, formatDuration } from "../_shared";

// Очередь звонков, помеченных фрод-предфильтром на транскрибации
// (fraud_review_status='pending') и ждущих решения человека — задача 7.
// Подтверждение/отклонение здесь ничего не исключает из оплаты само по
// себе: внешний расчёт смотрит на fraud_review_status='confirmed', это
// просто фиксация вердикта. Полный список полей и логика — см.
// backend/internal/transcribe/client.go (computeFraudPrecheck) и миграцию
// 034_zvonari_fraud_precheck.sql.

// Плеер в браузере, без скачивания — тот же паттерн, что в CallDrawer
// (_call_drawer.tsx), но самостоятельный на каждую карточку: здесь
// одновременно на экране много звонков, а не один раскрытый.
function InlineAudioPlayer({ callId }: { callId: string }) {
  const audioRef = useRef<HTMLAudioElement>(null);
  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);

  function togglePlay() {
    const audio = audioRef.current;
    if (!audio) return;
    if (playing) audio.pause();
    else audio.play().catch(() => {});
  }

  return (
    <div className="flex items-center gap-3 rounded-md border border-border bg-muted/40 px-3 py-2">
      <audio
        ref={audioRef}
        src={zvonariAPI.recordingUrl(callId)}
        preload="none"
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onTimeUpdate={(e) => setCurrentTime(e.currentTarget.currentTime)}
        onLoadedMetadata={(e) => setDuration(e.currentTarget.duration)}
        onEnded={() => setPlaying(false)}
      />
      <Button size="sm" className="h-8 w-8 shrink-0 rounded-full p-0" onClick={togglePlay} aria-label={playing ? "Пауза" : "Слушать"}>
        {playing ? <Pause className="h-3.5 w-3.5" /> : <Play className="h-3.5 w-3.5 translate-x-0.5" />}
      </Button>
      <div
        className="relative h-1.5 flex-1 cursor-pointer rounded-full bg-border"
        onClick={(e) => {
          const rect = e.currentTarget.getBoundingClientRect();
          const fraction = (e.clientX - rect.left) / rect.width;
          const audio = audioRef.current;
          if (audio) audio.currentTime = fraction * (duration || 0);
        }}
      >
        <div
          className="absolute left-0 top-0 h-full rounded-full bg-primary"
          style={{ width: duration ? `${Math.min(100, (currentTime / duration) * 100)}%` : "0%" }}
        />
      </div>
      <span className="whitespace-nowrap font-mono text-xs tabular-nums text-muted-foreground">
        {formatDuration(Math.floor(currentTime))} / {duration ? formatDuration(Math.floor(duration)) : "—:—"}
      </span>
    </div>
  );
}

export default function FraudReviewPage() {
  const [from, setFrom] = useState(todayISO(-6));
  const [to, setTo] = useState(todayISO());
  const [callers, setCallers] = useState<Caller[]>([]);
  const [candidates, setCandidates] = useState<Call[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  // reviewingId — id звонка, для которого сейчас летит запрос
  // confirm/reject, чтобы задизействовать именно его кнопки, а не все разом.
  const [reviewingId, setReviewingId] = useState<string | null>(null);
  // selectedCaller — "" значит "все звонари" (группы стопкой, как раньше);
  // конкретный id — навигация ограничивается одним звонарём, а не листание
  // длинного списка групп в поисках нужного.
  const [selectedCaller, setSelectedCaller] = useState<string>("");
  const [exporting, setExporting] = useState(false);

  useEffect(() => {
    zvonariAPI.getCallers().then((res) => setCallers(res.data || [])).catch(() => {});
  }, []);

  useEffect(() => {
    setLoading(true);
    setError("");
    zvonariAPI
      .getFraudCandidates(from, to)
      .then((res) => setCandidates(res.data || []))
      .catch((err: unknown) => {
        const message = err instanceof Error ? err.message : "Не удалось загрузить кандидатов";
        setError(message.replace(/\s*\(HTTP \d+\)$/, ""));
      })
      .finally(() => setLoading(false));
  }, [from, to]);

  // Группировка по звонарю — "без звонаря" (caller_id=null, нераспознанный
  // добавочный) отдельной группой в конце, а не потерян/смешан.
  const groups = useMemo(() => {
    const byId = new Map<string, Call[]>();
    for (const call of candidates) {
      const key = call.caller_id || "__none__";
      if (!byId.has(key)) byId.set(key, []);
      byId.get(key)!.push(call);
    }
    const rows = Array.from(byId.entries()).map(([callerId, calls]) => ({
      callerId,
      name: callerId === "__none__" ? "Без звонаря" : callers.find((c) => c.id === callerId)?.name || callerId,
      calls: calls.sort((a, b) => a.started_at.localeCompare(b.started_at)),
    }));
    // Больше кандидатов — выше: там, где разбор нужнее всего, видно сразу,
    // не нужно листать в поисках самой заваленной очереди.
    rows.sort((a, b) => b.calls.length - a.calls.length);
    return rows;
  }, [candidates, callers]);

  // Фильтр по звонарю — сужает groups до одной записи вместо переопределения
  // отдельным списком, чтобы разметка ниже (группа → карточки) не дублировалась.
  const visibleGroups = useMemo(
    () => (selectedCaller ? groups.filter((g) => g.callerId === selectedCaller) : groups),
    [groups, selectedCaller]
  );

  async function handleExport() {
    setExporting(true);
    setError("");
    try {
      const url = zvonariAPI.fraudExportXlsxUrl(from, to);
      const response = await fetch(url);
      if (!response.ok) {
        const text = await response.text().catch(() => "");
        throw new Error(text || `Ошибка HTTP ${response.status}`);
      }
      const disposition = response.headers.get("content-disposition");
      const match = disposition?.match(/filename\*=UTF-8''([^;]+)/i) || disposition?.match(/filename="?([^";]+)"?/i);
      const filename = match?.[1] ? decodeURIComponent(match[1]) : "fraud.xlsx";
      const blob = await response.blob();
      const blobUrl = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = blobUrl;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(blobUrl);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "Не удалось выгрузить файл";
      setError(message.replace(/\s*\(HTTP \d+\)$/, ""));
    } finally {
      setExporting(false);
    }
  }

  async function review(callId: string, status: "confirmed" | "rejected") {
    setReviewingId(callId);
    try {
      await zvonariAPI.reviewFraudCandidate(callId, status);
      // Решённые кандидаты уходят из очереди сразу — список выше всегда
      // "то, что ещё ждёт решения", а не архив прошлых решений.
      setCandidates((prev) => prev.filter((c) => c.id !== callId));
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "Не удалось сохранить решение";
      setError(message.replace(/\s*\(HTTP \d+\)$/, ""));
    } finally {
      setReviewingId(null);
    }
  }

  return (
    <div className="mx-auto max-w-5xl space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Link href="/zvonari">
            <Button variant="ghost" size="icon">
              <ArrowLeft className="h-4 w-4" />
            </Button>
          </Link>
          <div>
            <h1 className="flex items-center gap-2 text-xl font-semibold">
              <ShieldAlert className="h-5 w-5 text-destructive" />
              Очередь на проверку фрода
            </h1>
            <p className="text-sm text-muted-foreground">
              Звонки, помеченные алгоритмом на транскрибации, сгруппированы по звонарю — подтвердите или отклоните каждый вручную.
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Select
            className="w-48"
            value={selectedCaller}
            onChange={(e) => setSelectedCaller(e.target.value)}
          >
            <option value="">Все звонари</option>
            {groups.map((g) => (
              <option key={g.callerId} value={g.callerId}>
                {g.name} ({g.calls.length})
              </option>
            ))}
          </Select>
          <PeriodControl from={from} to={to} onChange={(f, t) => { setFrom(f); setTo(t); }} />
          <Button variant="outline" size="sm" onClick={handleExport} disabled={exporting}>
            {exporting ? <Loader2 className="mr-1.5 h-4 w-4 animate-spin" /> : <Download className="mr-1.5 h-4 w-4" />}
            Выгрузить xlsx
          </Button>
        </div>
      </div>

      {error && (
        <Card className="border-destructive/50">
          <CardContent className="py-3 text-sm text-destructive">{error}</CardContent>
        </Card>
      )}

      {loading ? (
        <div className="flex items-center justify-center py-16 text-muted-foreground">
          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          Загрузка...
        </div>
      ) : visibleGroups.length === 0 ? (
        <Card>
          <CardContent className="py-16 text-center text-sm text-muted-foreground">
            {selectedCaller ? "У этого звонаря кандидатов на фрод нет." : "За выбранный период кандидатов на фрод нет."}
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-8">
          {visibleGroups.map((group) => (
            <div key={group.callerId} className="space-y-3">
              <div className="flex items-center gap-2">
                <h2 className="text-sm font-semibold">{group.name}</h2>
                <Badge variant="secondary">{group.calls.length}</Badge>
              </div>
              {group.calls.map((call) => (
                <Card key={call.id}>
                  <CardHeader className="pb-3">
                    <CardTitle className="text-base font-medium">
                      {new Date(call.started_at).toLocaleString("ru-RU")}
                    </CardTitle>
                    <CardDescription className="flex items-center gap-2 text-xs">
                      <span>{call.talk_time_sec}с разговора / {call.duration_sec}с всего</span>
                      {call.fraud_precheck_position && (
                        <Badge variant="outline" className="text-xs">
                          позиция: {call.fraud_precheck_position}
                        </Badge>
                      )}
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <InlineAudioPlayer callId={call.id} />
                    {call.fraud_precheck_quote && (
                      <p className="rounded-md bg-destructive/10 p-2 text-sm text-destructive">
                        «{call.fraud_precheck_quote}»
                      </p>
                    )}
                    <p className="whitespace-pre-wrap rounded-md bg-muted/50 p-2 text-sm text-muted-foreground">
                      {call.transcript_text || "Транскрипт недоступен"}
                    </p>
                    <div className="flex justify-end gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={reviewingId === call.id}
                        onClick={() => review(call.id, "rejected")}
                      >
                        <XCircle className="mr-1.5 h-4 w-4" />
                        Не фрод
                      </Button>
                      <Button
                        variant="destructive"
                        size="sm"
                        disabled={reviewingId === call.id}
                        onClick={() => review(call.id, "confirmed")}
                      >
                        <CheckCircle2 className="mr-1.5 h-4 w-4" />
                        Подтвердить фрод
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
