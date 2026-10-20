"use client"
import { useCallback, useEffect, useState } from "react"
import { ExternalLink, Loader2, Sheet } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Alert } from "@/components/ui/alert"
import { GoogleAuthStatus, googleAuthAPI } from "@/lib/api"
import { cn } from "@/lib/utils"

// OAuth-приложение Google в тестовом режиме: refresh-токен живёт ~7 дней
// после согласия, поэтому заранее подсвечиваем, что скоро надо переподключить.
const TOKEN_LIFETIME_DAYS = 7
const WARN_AFTER_DAYS = 6

type Health = "loading" | "ok" | "expiring" | "down" | "unknown"

function daysSince(iso: string | null): number | null {
  if (!iso) return null
  const t = Date.parse(iso)
  if (Number.isNaN(t)) return null
  return (Date.now() - t) / 86_400_000
}

function healthOf(status: GoogleAuthStatus | null, failed: boolean): Health {
  if (failed) return "unknown"
  if (!status) return "loading"
  if (!status.authenticated) return "down"
  const age = daysSince(status.authorized_at)
  return age !== null && age >= WARN_AFTER_DAYS ? "expiring" : "ok"
}

function stripHttp(message: string): string {
  return message.replace(/\s*\(HTTP \d+\)$/, "")
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString("ru-RU", { day: "numeric", month: "long", hour: "2-digit", minute: "2-digit" })
}

const STATE_TEXT: Record<string, string> = {
  NOT_AUTHENTICATED: "Google не подключён",
  TOKEN_REVOKED: "Токен истёк или отозван",
  REFRESH_FAILED: "Не удалось обновить токен",
  TOKEN_CORRUPT: "Файл токена повреждён",
  TOKEN_INVALID: "Токен недействителен",
  OAUTH_CLIENT_DISABLED: "OAuth-клиент Google отключён",
}

export default function GoogleAuthDialog() {
  const [isOpen, setIsOpen] = useState(false)
  const [status, setStatus] = useState<GoogleAuthStatus | null>(null)
  const [statusFailed, setStatusFailed] = useState(false)
  const [authUrl, setAuthUrl] = useState("")
  const [pasted, setPasted] = useState("")
  const [busy, setBusy] = useState<"" | "url" | "code">("")
  const [error, setError] = useState("")
  const [justConnected, setJustConnected] = useState(false)

  const loadStatus = useCallback(async () => {
    try {
      const response = await googleAuthAPI.getStatus()
      setStatus(response.data)
      setStatusFailed(false)
    } catch (err) {
      console.error("Failed to load Google auth status:", err)
      setStatusFailed(true)
    }
  }, [])

  useEffect(() => {
    loadStatus()
  }, [loadStatus])

  const health = healthOf(status, statusFailed)

  const handleOpenChange = (open: boolean) => {
    setIsOpen(open)
    if (open) {
      setError("")
      setJustConnected(false)
      loadStatus()
    } else {
      setAuthUrl("")
      setPasted("")
    }
  }

  const handleGetLink = async () => {
    setBusy("url")
    setError("")
    try {
      const response = await googleAuthAPI.getAuthUrl()
      setAuthUrl(response.data.url)
      setPasted("")
      window.open(response.data.url, "_blank", "noopener,noreferrer")
    } catch (err) {
      setError(err instanceof Error ? stripHttp(err.message) : "Не удалось получить ссылку")
    } finally {
      setBusy("")
    }
  }

  const handleSubmit = async () => {
    if (!pasted.trim()) return
    setBusy("code")
    setError("")
    try {
      const response = await googleAuthAPI.submitCode(pasted.trim())
      setStatus(response.data)
      setStatusFailed(false)
      setAuthUrl("")
      setPasted("")
      setJustConnected(true)
    } catch (err) {
      setError(err instanceof Error ? stripHttp(err.message) : "Не удалось подключить Google")
    } finally {
      setBusy("")
    }
  }

  const age = daysSince(status?.authorized_at ?? null)
  const daysLeft = age === null ? null : Math.max(0, Math.ceil(TOKEN_LIFETIME_DAYS - age))

  return (
    <Dialog open={isOpen} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button
          variant="outline"
          className={cn(
            health === "down" && "border-destructive/50 text-destructive",
            health === "expiring" && "border-warning/50 text-warning",
          )}
          title="Подключение к Google-таблицам (номера счетов и актов)"
        >
          <span
            className={cn(
              "mr-2 h-2 w-2 rounded-full",
              health === "ok" && "bg-success",
              health === "expiring" && "bg-warning",
              health === "down" && "bg-destructive",
              (health === "loading" || health === "unknown") && "bg-muted-foreground/40",
            )}
          />
          <Sheet className="mr-2 h-4 w-4" />
          {health === "down" ? "Подключить Google" : "Google-таблицы"}
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Google-таблицы</DialogTitle>
          <DialogDescription>
            Через это подключение берутся номера счетов и актов из реестров. Пока OAuth-приложение в тестовом
            режиме, Google отключает его примерно через {TOKEN_LIFETIME_DAYS} дней — тогда подключите заново.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {statusFailed ? (
            <Alert>Не удалось получить статус: сервис sheets-sync недоступен.</Alert>
          ) : !status ? (
            <div className="flex items-center text-sm text-muted-foreground">
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Проверяю подключение…
            </div>
          ) : status.authenticated ? (
            <Alert variant={health === "expiring" ? "warning" : "success"}>
              {justConnected ? "Google подключён." : "Подключено."}
              {status.authorized_at && (
                <>
                  {" "}Последнее подключение — {formatDate(status.authorized_at)}
                  {daysLeft !== null && (daysLeft > 0 ? `, осталось примерно ${daysLeft} дн.` : ", срок вот-вот истечёт.")}
                </>
              )}
            </Alert>
          ) : (
            <Alert>
              {STATE_TEXT[status.state] ?? "Google не подключён"}. Номера из таблиц сейчас не подтягиваются.
              {!status.has_client_secret && " На сервере нет client secret — подключение из интерфейса невозможно."}
            </Alert>
          )}

          <ol className="list-decimal space-y-3 pl-5 text-sm">
            <li>
              <div className="mb-2">Откройте ссылку и войдите в Google-аккаунт, у которого есть доступ к таблицам.</div>
              <div className="flex flex-wrap items-center gap-2">
                <Button onClick={handleGetLink} disabled={busy !== "" || status?.has_client_secret === false} size="sm">
                  {busy === "url" ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <ExternalLink className="mr-2 h-4 w-4" />}
                  {authUrl ? "Получить новую ссылку" : "Открыть вход в Google"}
                </Button>
                {authUrl && (
                  <a href={authUrl} target="_blank" rel="noopener noreferrer" className="text-sm underline text-muted-foreground">
                    открыть ещё раз
                  </a>
                )}
              </div>
            </li>
            <li>
              <div className="mb-2">
                После «Разрешить» браузер откроет пустую страницу с ошибкой <span className="font-mono">localhost:1</span> — так
                и должно быть. Скопируйте адрес из адресной строки целиком и вставьте сюда:
              </div>
              <Input
                placeholder="http://localhost:1/?state=…&code=…"
                aria-label="Адрес из браузера после входа в Google"
                value={pasted}
                onChange={(e) => setPasted(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleSubmit()
                }}
                disabled={!authUrl || busy !== ""}
                className="font-mono text-xs"
              />
            </li>
          </ol>

          {error && <Alert>{error}</Alert>}
        </div>

        <DialogFooter>
          <Button onClick={handleSubmit} disabled={!authUrl || !pasted.trim() || busy !== ""}>
            {busy === "code" && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Подключить
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
