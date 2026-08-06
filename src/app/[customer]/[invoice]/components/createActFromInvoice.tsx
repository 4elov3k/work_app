"use client"
import React, { useCallback, useEffect, useState } from "react"
import { Loader2, Wand2 } from "lucide-react"
import { useRouter } from "next/navigation"

import { contractsAPI, invoicesAPI } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
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

const DUPLICATE_NUMBER_MESSAGE = "Номер счета совпадает с уже существующим номером акта по этому договору"

type Props = {
  invoiceId: string
  customerId: string
  contractId: string
}

export default function CreateActFromInvoice({ invoiceId, customerId, contractId }: Props) {
  const router = useRouter()
  const [open, setOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [loadingNumber, setLoadingNumber] = useState(false)
  const [number, setNumber] = useState("")
  const [date, setDate] = useState("")
  const [error, setError] = useState("")

  const loadNextNumber = useCallback(async () => {
    setLoadingNumber(true)
    setError("")
    try {
      const response = await contractsAPI.getNextDocNumber(contractId, "act")
      setNumber(response.number)
    } catch (err: unknown) {
      console.error("Failed to load next act number:", err)
      const message = err instanceof Error ? err.message : "Не удалось получить номер акта"
      setError(message)
    } finally {
      setLoadingNumber(false)
    }
  }, [contractId])

  useEffect(() => {
    if (open && !number) {
      loadNextNumber()
    }
  }, [loadNextNumber, number, open])

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
      const message = err instanceof Error ? err.message : "Ошибка при создании акта"
      if (message.includes("HTTP 409") || message.includes("already exists")) {
        setError(DUPLICATE_NUMBER_MESSAGE)
      } else {
        console.error("Failed to create act from invoice:", err)
        setError(message)
      }
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
          {error && <Alert>{error}</Alert>}
          <div className="grid gap-4 py-4">
            <div className="space-y-2">
              <label htmlFor="actNumber" className="text-sm font-medium">
                Номер акта
              </label>
              <Input id="actNumber" value={number} onChange={(e) => setNumber(e.target.value)} required disabled={loadingNumber} />
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
            <Button type="submit" disabled={submitting || loadingNumber}>
              {submitting || loadingNumber ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  {loadingNumber ? "Загрузка..." : "Создание..."}
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
