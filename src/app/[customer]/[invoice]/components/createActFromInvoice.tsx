"use client"
import React, { useState } from "react"
import { Loader2, Wand2 } from "lucide-react"
import { useRouter } from "next/navigation"

import { invoicesAPI } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
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
  invoiceId: string
  customerId: string
}

export default function CreateActFromInvoice({ invoiceId, customerId }: Props) {
  const router = useRouter()
  const [open, setOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [number, setNumber] = useState("")
  const [date, setDate] = useState("")
  const [error, setError] = useState("")

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    setSubmitting(true)
    setError("")
    try {
      const formattedDate = date.split("-").reverse().join(".")
      const response = await invoicesAPI.createActFromInvoice(invoiceId, { number, date: formattedDate })
      setOpen(false)
      setNumber("")
      setDate("")
      router.push(`/${customerId}/acts/${response.data.id}`)
    } catch (err: unknown) {
      console.error("Failed to create act from invoice:", err)
      const message = err instanceof Error ? err.message : "Ошибка при создании акта"
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline">
          <Wand2 className="mr-2 h-4 w-4" />
          Сформировать акт
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Акт на основании счета</DialogTitle>
            <DialogDescription>Укажите номер и дату акта.</DialogDescription>
          </DialogHeader>
          {error && (
            <div className="bg-red-50 border border-red-200 text-red-800 px-4 py-3 rounded text-sm">
              {error}
            </div>
          )}
          <div className="grid gap-4 py-4">
            <div className="space-y-2">
              <label htmlFor="actNumber" className="text-sm font-medium">
                Номер акта
              </label>
              <Input id="actNumber" value={number} onChange={(e) => setNumber(e.target.value)} required />
            </div>
            <div className="space-y-2">
              <label htmlFor="actDate" className="text-sm font-medium">
                Дата акта
              </label>
              <Input id="actDate" type="date" value={date} onChange={(e) => setDate(e.target.value)} required />
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setOpen(false)} disabled={submitting}>
              Отмена
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Создание...
                </>
              ) : (
                "Создать"
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
