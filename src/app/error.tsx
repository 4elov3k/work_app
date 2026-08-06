"use client"

import { useEffect } from "react"
import Link from "next/link"
import { AlertTriangle } from "lucide-react"
import { Button } from "@/components/ui/button"

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  useEffect(() => {
    console.error("Unhandled page error:", error)
  }, [error])

  return (
    <div className="container mx-auto flex min-h-[60vh] max-w-xl flex-col items-center justify-center gap-4 px-4 text-center">
      <AlertTriangle className="h-12 w-12 text-red-500" />
      <h1 className="text-2xl font-bold tracking-tight">Что-то пошло не так</h1>
      <p className="text-muted-foreground">
        {error.message || "Не удалось загрузить страницу. Попробуйте ещё раз или вернитесь на главную."}
      </p>
      <div className="flex gap-3">
        <Button onClick={() => reset()}>Повторить</Button>
        <Link href="/">
          <Button variant="outline">На главную</Button>
        </Link>
      </div>
    </div>
  )
}
