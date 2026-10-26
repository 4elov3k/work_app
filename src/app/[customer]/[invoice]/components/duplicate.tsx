"use client"
import { useState } from "react"
import { Copy, Loader2, RefreshCw } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Select } from "@/components/ui/select"
import { Alert } from "@/components/ui/alert"
import { invoicesAPI } from "@/lib/api"
import { useRouter } from "next/navigation"

interface DuplicateProps {
  invoiceId: string
  customerId: string
}

export default function Duplicate({ invoiceId, customerId }: DuplicateProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [number, setNumber] = useState("")
  const [date, setDate] = useState("")
  const [error, setError] = useState("")
  const [sheetSyncing, setSheetSyncing] = useState(false)
  const [syncedWithSheet, setSyncedWithSheet] = useState(false)
  const [sheetNote, setSheetNote] = useState("")
  // Категория проекта для реестра счетов — ручной выбор, как при создании счета.
  const [invoiceCategory, setInvoiceCategory] = useState("")
  const router = useRouter()

  const handleOpenChange = (value: boolean) => {
    setIsOpen(value)
    if (value) setError("")
  }

  const handleSyncWithSheet = async () => {
    setSheetSyncing(true)
    setError("")
    setSheetNote("")
    try {
      const res = await invoicesAPI.getNextNumberFromSheet()
      setNumber(res.data.number)
      setSyncedWithSheet(true)
      setSheetNote(`Номер ${res.data.number} взят из таблицы (строка ${res.data.row}). При создании копии строка будет дописана автоматически.`)
    } catch (err: unknown) {
      console.error("Failed to sync invoice number with sheet:", err)
      const message = err instanceof Error ? err.message : "Не удалось синхронизироваться с таблицей"
      setError(message.replace(/\s*\(HTTP \d+\)$/, ""))
    } finally {
      setSheetSyncing(false)
    }
  }

  const handleDuplicate = async (event: React.FormEvent) => {
    event.preventDefault()
    setError("")

    if (syncedWithSheet && !invoiceCategory) {
      setError("Выберите категорию проекта для реестра")
      return
    }

    setSubmitting(true)
    try {
      const newDate = date.split("-").reverse().join(".")
      const response = await invoicesAPI.duplicate({
        invoice_id: invoiceId,
        number: number,
        date: newDate
      })

      if (syncedWithSheet) {
        try {
          await invoicesAPI.registerInSheet(response.data.id, invoiceCategory)
        } catch (sheetErr: unknown) {
          console.error("Failed to register invoice in sheet:", sheetErr)
          const sheetMessage = sheetErr instanceof Error ? sheetErr.message : "неизвестная ошибка"
          window.alert(`Копия создана, но не удалось дописать её в таблицу: ${sheetMessage.replace(/\s*\(HTTP \d+\)$/, "")}`)
        }
      }

      setIsOpen(false)
      setNumber("")
      setDate("")
      setSyncedWithSheet(false)
      setSheetNote("")
      setInvoiceCategory("")

      // Перенаправляем на новый счет
      router.push(`/${customerId}/${response.data.id}`)
      router.refresh()
    } catch (err) {
      console.error('Failed to duplicate invoice:', err)
      setError(err instanceof Error ? err.message : 'Ошибка при дублировании счета')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={isOpen} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button variant="secondary">
          <Copy className="mr-2 h-4 w-4" />
          Дублировать
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={handleDuplicate}>
          <DialogHeader>
            <DialogTitle>Дублировать документ</DialogTitle>
            <DialogDescription>
              Будет создан новый документ с теми же услугами
            </DialogDescription>
          </DialogHeader>
          {error && <Alert>{error}</Alert>}
          <div className="pt-2">
            <Button type="button" variant="outline" size="sm" onClick={handleSyncWithSheet} disabled={sheetSyncing}>
              {sheetSyncing ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Синхронизация...
                </>
              ) : (
                <>
                  <RefreshCw className="mr-2 h-4 w-4" />
                  Синхронизировать с таблицей
                </>
              )}
            </Button>
            {sheetNote && <p className="text-xs text-muted-foreground mt-2">{sheetNote}</p>}
            {syncedWithSheet && (
              <div className="space-y-2 mt-3">
                <label htmlFor="duplicateInvoiceCategory" className="text-sm font-medium">
                  Категория проекта (для реестра)
                </label>
                <Select
                  id="duplicateInvoiceCategory"
                  value={invoiceCategory}
                  onChange={(e) => setInvoiceCategory(e.target.value)}
                  required
                >
                  <option value="">Выберите категорию...</option>
                  <option value="Продвижение">Продвижение</option>
                  <option value="Разработка">Разработка</option>
                  <option value="Хостинг">Хостинг</option>
                </Select>
              </div>
            )}
          </div>
          <div className="grid gap-4 py-4">
            <div className="space-y-2">
              <label htmlFor="number" className="text-sm font-medium">
                Новый номер
              </label>
              <Input
                id="number"
                placeholder="002"
                value={number}
                onChange={(e) => {
                  setNumber(e.target.value)
                  setSyncedWithSheet(false)
                  setSheetNote("")
                  setInvoiceCategory("")
                }}
                required
              />
            </div>
            <div className="space-y-2">
              <label htmlFor="date" className="text-sm font-medium">
                Новая дата
              </label>
              <Input
                id="date"
                type="date"
                value={date}
                onChange={(e) => setDate(e.target.value)}
                required
              />
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setIsOpen(false)} disabled={submitting}>
              Отмена
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Дублирование...
                </>
              ) : (
                <>
                  <Copy className="mr-2 h-4 w-4" />
                  Создать копию
                </>
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
