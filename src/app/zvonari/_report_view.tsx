"use client"
// Отчёт по звонарю за период (2026-10-13): вместо одного абзаца текста —
// качества (4 критерия рубрики) с вердиктом по периоду, что делать
// руководителю, звонки для разбора с плеером и фокус на следующий период.
//
// Главное в карточке качества — вердикт модели по периоду, а не процент:
// бинарная оценка звонка почти никогда не засчитывает критерий (у всех
// звонарей 0–12%, даже в разговорах дольше минуты), так что процент не
// различает людей. Он показан мелко, для справки.
import { useState } from "react";
import { CheckCircle2, ClipboardList, Lightbulb, PlayCircle, Search, Target, TrendingUp, TriangleAlert } from "lucide-react";

import { Call, CallerReport, CriterionKey, ReportCallRef, zvonariAPI } from "@/lib/api";
import { CallDrawer } from "./_call_drawer";

const CRITERIA: { key: CriterionKey; label: string }[] = [
  { key: "listening", label: "Умение слушать" },
  { key: "goal_understanding", label: "Понимание цели разговора" },
  { key: "questions", label: "Умение задавать вопросы" },
  { key: "objection_handling", label: "Работа с возражениями" },
];

function verdictStyle(verdict?: string | null): { className: string; label: string } {
  const v = (verdict ?? "").toLowerCase();
  if (v.includes("сильн")) return { className: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300", label: "Сильная сторона" };
  if (v.includes("норм")) return { className: "bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-300", label: "В норме" };
  if (v.includes("рост")) return { className: "bg-amber-100 text-amber-900 dark:bg-amber-950 dark:text-amber-300", label: "Зона роста" };
  return { className: "bg-muted text-muted-foreground", label: verdict ? verdict : "Мало данных" };
}

// Модель иногда пишет в тексте ключи критериев (listening — 1%…); в промпте
// это запрещено, но старые отчёты уже сохранены с ними — подменяем на лету.
function ru(text?: string | null): string {
  let out = text ?? "";
  for (const { key, label } of CRITERIA) out = out.replace(new RegExp(`\\b${key}\\b`, "g"), `«${label.toLowerCase()}»`);
  return out;
}

function callLabel(ref: ReportCallRef): string {
  const d = new Date(ref.started_at);
  const when = isNaN(d.getTime())
    ? ""
    : d.toLocaleString("ru-RU", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
  return `Звонок ${ref.number}${when ? `, ${when}` : ""}`;
}

// CallRef — звонок, на который ссылается отчёт: «Послушать» — плеер прямо
// здесь; «Разбор» — та же панель, что в списке звонков, если звонок есть в
// загруженном периоде.
function CallRef({ refCall, onOpen, canOpen }: { refCall: ReportCallRef; onOpen: (id: string) => void; canOpen: boolean }) {
  const [listening, setListening] = useState(false);
  return (
    <span className="inline-flex flex-col gap-1">
      <span className="inline-flex flex-wrap items-center gap-2 text-xs">
        <span className="font-medium text-foreground">{callLabel(refCall)}</span>
        <button type="button" onClick={() => setListening((v) => !v)} className="inline-flex items-center gap-1 text-primary hover:underline">
          <PlayCircle className="h-3.5 w-3.5" />
          {listening ? "Скрыть" : "Послушать"}
        </button>
        {canOpen && (
          <button type="button" onClick={() => onOpen(refCall.call_id)} className="inline-flex items-center gap-1 text-primary hover:underline">
            <Search className="h-3.5 w-3.5" />
            Разбор
          </button>
        )}
      </span>
      {listening && <audio controls autoPlay preload="none" src={zvonariAPI.recordingUrl(refCall.call_id)} className="h-8 w-72" />}
    </span>
  );
}

function Section({ icon, title, children }: { icon: React.ReactNode; title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-2">
      <h4 className="flex items-center gap-1.5 text-sm font-semibold">
        {icon}
        {title}
      </h4>
      {children}
    </section>
  );
}

export function ReportView({
  report,
  calls,
  onAnalyze,
  analyzingIds,
}: {
  report: CallerReport;
  calls: Call[];
  onAnalyze: (callId: string) => void;
  analyzingIds: Set<string>;
}) {
  const [openCallId, setOpenCallId] = useState<string | null>(null);
  const m = report.metrics_json ?? {};
  const r = m.report;
  const byId = new Map(calls.map((c) => [c.id, c]));
  const openCall = openCallId ? byId.get(openCallId) ?? null : null;
  const refProps = (ref: ReportCallRef) => ({ refCall: ref, onOpen: setOpenCallId, canOpen: byId.has(ref.call_id) });

  const pct = (k: CriterionKey) => {
    const c = m.by_criterion?.[k];
    return c && c.applicable ? { text: `${Math.round((100 * c.met) / c.applicable)}% (${c.met} из ${c.applicable})`, value: (100 * c.met) / c.applicable } : null;
  };

  return (
    <div className="space-y-6">
      <p className="text-xs text-muted-foreground">
        Период {report.period_start} — {report.period_end}
        {m.total_calls != null && <> · звонков {m.total_calls}{m.scored_calls != null && `, с оценкой ${m.scored_calls}`}</>}
        {" · "}сформирован {new Date(report.requested_at).toLocaleString("ru-RU")}
      </p>

      {r?.summary ? (
        <p className="text-sm leading-relaxed">{ru(r.summary)}</p>
      ) : (
        <p className="whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">{ru(report.summary_text)}</p>
      )}

      {(r?.criteria || m.by_criterion) && (
        <Section icon={<CheckCircle2 className="h-4 w-4 text-primary" />} title="Качества">
          <div className="grid gap-3 md:grid-cols-2">
            {CRITERIA.map(({ key, label }) => {
              const c = r?.criteria?.[key];
              const v = verdictStyle(c?.verdict);
              const p = pct(key);
              return (
                <div key={key} className="space-y-2 rounded-md border border-border p-3">
                  <div className="flex items-start justify-between gap-2">
                    <span className="text-sm font-medium">{label}</span>
                    {c && <span className={`shrink-0 rounded px-2 py-0.5 text-xs font-medium ${v.className}`}>{v.label}</span>}
                  </div>
                  {c?.comment && <p className="text-sm text-muted-foreground">{ru(c.comment)}</p>}
                  {c?.example_call && <CallRef {...refProps(c.example_call)} />}
                  {p && (
                    <div className="space-y-1" title="Доля звонков, где качество засчитано по оценке отдельного звонка, из звонков, где была ситуация">
                      <div className="h-1.5 overflow-hidden rounded bg-muted">
                        <div className="h-1.5 rounded bg-primary/60" style={{ width: `${Math.max(p.value, 1.5)}%` }} />
                      </div>
                      <p className="text-[11px] text-muted-foreground">По оценкам звонков: {p.text}</p>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </Section>
      )}

      {!!r?.manager_actions?.length && (
        <Section icon={<ClipboardList className="h-4 w-4 text-primary" />} title="Что делать руководителю">
          <ol className="space-y-3">
            {r.manager_actions.map((a, i) => (
              <li key={i} className="flex gap-3 rounded-md border border-border p-3">
                <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary text-xs font-semibold text-primary-foreground">
                  {i + 1}
                </span>
                <div className="space-y-1.5">
                  <p className="text-sm font-medium">{ru(a.action)}</p>
                  {a.why && <p className="text-sm text-muted-foreground"><span className="font-medium text-foreground">Зачем: </span>{ru(a.why)}</p>}
                  {a.how && <p className="text-sm text-muted-foreground"><span className="font-medium text-foreground">Как: </span>{ru(a.how)}</p>}
                  {!!a.calls?.length && (
                    <div className="flex flex-wrap gap-x-4 gap-y-1 pt-1">
                      {a.calls.map((ref) => <CallRef key={ref.call_id} {...refProps(ref)} />)}
                    </div>
                  )}
                </div>
              </li>
            ))}
          </ol>
        </Section>
      )}

      {(!!r?.strengths?.length || !!r?.growth_areas?.length) && (
        <div className="grid gap-6 md:grid-cols-2">
          <Section icon={<TrendingUp className="h-4 w-4 text-emerald-600" />} title="Сильные стороны">
            <ul className="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
              {(r?.strengths ?? []).map((s, i) => <li key={i}>{ru(s)}</li>)}
            </ul>
          </Section>
          <Section icon={<TriangleAlert className="h-4 w-4 text-amber-600" />} title="Зоны роста">
            <ul className="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
              {(r?.growth_areas ?? []).map((s, i) => <li key={i}>{ru(s)}</li>)}
            </ul>
          </Section>
        </div>
      )}

      {!!r?.calls_to_review?.length && (
        <Section icon={<Lightbulb className="h-4 w-4 text-primary" />} title="Звонки для разбора">
          <ul className="space-y-3">
            {r.calls_to_review.map((c) => (
              <li key={c.call_id} className="space-y-1 rounded-md border border-border p-3">
                <CallRef {...refProps(c)} />
                {c.why && <p className="text-sm text-muted-foreground">{ru(c.why)}</p>}
              </li>
            ))}
          </ul>
        </Section>
      )}

      {r?.focus_next && (
        <div className="flex gap-3 rounded-md border border-primary/30 bg-primary/5 p-3">
          <Target className="mt-0.5 h-4 w-4 shrink-0 text-primary" />
          <div>
            <p className="text-sm font-semibold">Фокус на следующий период</p>
            <p className="text-sm text-muted-foreground">{ru(r.focus_next)}</p>
          </div>
        </div>
      )}

      <CallDrawer
        call={openCall}
        calls={openCall ? [openCall] : []}
        initialTab="transcript"
        onClose={() => setOpenCallId(null)}
        onNavigate={() => {}}
        onAnalyze={onAnalyze}
        isAnalyzing={openCall ? analyzingIds.has(openCall.id) : false}
      />
    </div>
  );
}

// ReportPreview — короткая карточка последнего отчёта на вкладке «Обзор»:
// вердикты по качествам и первое действие для руководителя.
export function ReportPreview({ report }: { report: CallerReport }) {
  const r = report.metrics_json?.report;
  if (!r) {
    return <p className="line-clamp-6 whitespace-pre-wrap text-sm text-muted-foreground">{ru(report.summary_text)}</p>;
  }
  const first = r.manager_actions?.[0];
  return (
    <div className="space-y-3">
      {r.summary && <p className="line-clamp-4 text-sm text-muted-foreground">{ru(r.summary)}</p>}
      <div className="flex flex-wrap gap-1.5">
        {CRITERIA.map(({ key, label }) => {
          const v = verdictStyle(r.criteria?.[key]?.verdict);
          return (
            <span key={key} className={`rounded px-2 py-0.5 text-xs ${v.className}`} title={v.label}>
              {label}: {v.label.toLowerCase()}
            </span>
          );
        })}
      </div>
      {first?.action && (
        <p className="text-sm">
          <span className="font-medium">Руководителю: </span>
          <span className="text-muted-foreground">{ru(first.action)}</span>
        </p>
      )}
    </div>
  );
}

