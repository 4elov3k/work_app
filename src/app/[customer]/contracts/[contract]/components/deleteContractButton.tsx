"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { Loader2, Trash2 } from "lucide-react"

import { contractsAPI } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Alert } from "@/components/ui/alert"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

export default function DeleteContractButton({
  customerId,
  contractId,
  contractNumber,
}: {
  customerId: string
  contractId: string
  contractNumber: string
}) {
  const router = useRouter()
  const [open, setOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [error, setError] = useState("")

  const handleDelete = async () => {
    setDeleting(true)
    setError("")
    try {
      await contractsAPI.delete(contractId)
      router.push(`/${customerId}`)
      router.refresh()
    } catch (err: unknown) {
      console.error("Failed to delete contract:", err)
      const message = err instanceof Error ? err.message : "Ошибка при удалении договора"
      if (message.includes("HTTP 409")) {
        setError("Нельзя удалить: у договора есть акты, счета или приложения. Сначала удалите их.")
      } else {
        setError(message.replace(/\s*\(HTTP \d+\)$/, ""))
      }
      setDeleting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Button
        variant="ghost"
        size="icon"
        className="text-muted-foreground hover:text-destructive"
        onClick={() => setOpen(true)}
      >
        <Trash2 className="h-4 w-4" />
      </Button>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Удалить договор?</DialogTitle>
          <DialogDescription>
            Договор № {contractNumber} будет удалён без возможности восстановления. Это действие нельзя отменить.
          </DialogDescription>
        </DialogHeader>
        {error && <Alert>{error}</Alert>}
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => setOpen(false)} disabled={deleting}>
            Отмена
          </Button>
          <Button type="button" variant="destructive" onClick={handleDelete} disabled={deleting}>
            {deleting ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Удаление...
              </>
            ) : (
              "Удалить"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
