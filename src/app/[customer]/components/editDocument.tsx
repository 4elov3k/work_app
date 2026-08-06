"use client"
import React, { useMemo, useState } from "react"
import { Loader2, Pencil } from "lucide-react"
import { useRouter } from "next/navigation"

import { invoicesAPI, actsAPI } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select } from "@/components/ui/select"
import { Alert } from "@/components/ui/alert"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"

type Props = {
  docType: "invoice" | "act"
  docId: string
  number: string
  date: string
  status: string
  archived: boolean
}

const STATUS_OPTIONS: Record<"invoice" | "act", { value: string; label: string }[]> = {
  invoice: [
    { value: "draft", label: "Черновик" },
    { value: "issued", label: "Выставлен" },
    { value: "paid", label: "Оплачен" },
    { value: "canceled", label: "Отменен" },
  ],
  act: [
    { value: "draft", label: "Черновик" },
    { value: "signed", label: "Подписан" },
    { value: "canceled", label: "Отменен" },
  ],
}

function toInputDate(ddmmyyyy: string) {
  const parts = ddmmyyyy.split(".")
  if (parts.length !== 3) return ""
  return `${parts[2]}-${parts[1]}-${parts[0]}`
}

function toApiDate(yyyymmdd: string) {
  const parts = yyyymmdd.split("-")
  if (parts.length !== 3) return ""
  return `${parts[2]}.${parts[1]}.${parts[0]}`
}

export default function EditDocument({ docType, docId, number, date, status, archived }: Props) {
  const router = useRouter()
  const [open, setOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [formNumber, setFormNumber] = useState(number)
  const [formDate, setFormDate] = useState(toInputDate(date))
  const [formStatus, setFormStatus] = useState(status)
  const [formArchived, setFormArchived] = useState(archived)
  const [error, setError] = useState("")

  const options = useMemo(() => STATUS_OPTIONS[docType], [docType])

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    setSubmitting(true)
    setError("")
    try {
      const payload: { number?: string; date?: string; status?: string; archived?: boolean } = {}
      if (!archived) {
        payload.number = formNumber
        payload.date = toApiDate(formDate)
        payload.status = formStatus
        payload.archived = formArchived
      } else {
        payload.archived = false
      }

      if (docType === "invoice") {
        await invoicesAPI.update(docId, payload)
      } else {
        await actsAPI.update(docId, payload)
      }
      setOpen(false)
      router.refresh()
    } catch (err: unknown) {
      console.error("Failed to update document:", err)
      const message = err instanceof Error ? err.message : "Ошибка при обновлении"
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline">
          <Pencil className="mr-2 h-4 w-4" />
          Редактировать
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Редактировать</DialogTitle>
            <DialogDescription>Измените параметры документа.</DialogDescription>
          </DialogHeader>
          {archived && (
            <Alert variant="warning">
              Документ в архиве, поля недоступны для правки. Снимите документ с архива, чтобы редактировать.
            </Alert>
          )}
          {error && <Alert>{error}</Alert>}
          <div className="grid gap-4 py-4">
            <div className="space-y-2">
              <label htmlFor="docNumber" className="text-sm font-medium">
                Номер
              </label>
              <Input
                id="docNumber"
                value={formNumber}
                onChange={(e) => setFormNumber(e.target.value)}
                disabled={archived}
                required
              />
            </div>
            <div className="space-y-2">
              <label htmlFor="docDate" className="text-sm font-medium">
                Дата
              </label>
              <Input
                id="docDate"
                type="date"
                value={formDate}
                onChange={(e) => setFormDate(e.target.value)}
                disabled={archived}
                required
              />
            </div>
            <div className="space-y-2">
              <label htmlFor="docStatus" className="text-sm font-medium">
                Статус
              </label>
              <Select
                id="docStatus"
                value={formStatus}
                onChange={(e) => setFormStatus(e.target.value)}
                disabled={archived}
              >
                {options.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </Select>
            </div>
            {!archived && (
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={formArchived}
                  onChange={(e) => setFormArchived(e.target.checked)}
                />
                В архиве
              </label>
            )}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setOpen(false)} disabled={submitting}>
              Отмена
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Сохранение...
                </>
              ) : archived ? (
                "Вывести из архива"
              ) : (
                "Сохранить"
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
