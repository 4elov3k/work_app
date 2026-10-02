"use client"
import { useState } from "react"
import { useRouter } from "next/navigation"
import { Loader2, Stamp } from "lucide-react"

import { invoicesAPI, actsAPI } from "@/lib/api"
import { Button } from "@/components/ui/button"

type Props = {
  docType: "invoice" | "act"
  docId: string
  signed: boolean
}

export default function SignDocument({ docType, docId, signed }: Props) {
  const router = useRouter()
  const [submitting, setSubmitting] = useState(false)

  const handleToggle = async () => {
    setSubmitting(true)
    try {
      if (docType === "invoice") {
        await invoicesAPI.sign(docId, !signed)
      } else {
        await actsAPI.sign(docId, !signed)
      }
      router.refresh()
    } catch (err) {
      console.error("Failed to toggle document signature:", err)
      alert("Не удалось изменить подпись документа")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Button variant={signed ? "outline" : "default"} onClick={handleToggle} disabled={submitting}>
      {submitting ? (
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
      ) : (
        <Stamp className="mr-2 h-4 w-4" />
      )}
      {signed ? "Отменить подпись" : "Подписать"}
    </Button>
  )
}
