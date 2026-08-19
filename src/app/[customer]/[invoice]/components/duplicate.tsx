"use client"
import { useState } from "react"
import { Copy, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
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
  const router = useRouter()

  const handleOpenChange = (value: boolean) => {
    setIsOpen(value)
    if (value) setError("")
  }

  const handleDuplicate = async (event: React.FormEvent) => {
    event.preventDefault()
    setSubmitting(true)
    setError("")

    try {
      const newDate = date.split("-").reverse().join(".")
      const response = await invoicesAPI.duplicate({
        invoice_id: invoiceId,
        number: number,
        date: newDate
      })

      setIsOpen(false)
      setNumber("")
      setDate("")
      
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
          <div className="grid gap-4 py-4">
            {error && <Alert>{error}</Alert>}
            <div className="space-y-2">
              <label htmlFor="number" className="text-sm font-medium">
                Новый номер
              </label>
              <Input
                id="number"
                placeholder="002"
                value={number}
                onChange={(e) => setNumber(e.target.value)}
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
