"use client"

import { useState } from "react"
import { Download } from "lucide-react"
import { Button } from "@/components/ui/button"

interface DownloadXmlProps {
  actId: string
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8080/api"

function filenameFromDisposition(disposition: string | null) {
  if (!disposition) return "upd.xml"

  const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (utf8Match?.[1]) {
    return decodeURIComponent(utf8Match[1])
  }

  const asciiMatch = disposition.match(/filename="?([^";]+)"?/i)
  return asciiMatch?.[1] || "upd.xml"
}

export default function DownloadXml({ actId }: DownloadXmlProps) {
  const [loading, setLoading] = useState(false)

  const handleDownload = async () => {
    setLoading(true)
    try {
      const response = await fetch(`${API_BASE}/acts/${actId}/export/upd-xml`)
      if (!response.ok) {
        const errorText = await response.text().catch(() => "")
        throw new Error(errorText || `Ошибка HTTP ${response.status}`)
      }

      const blob = await response.blob()
      const url = URL.createObjectURL(blob)
      const link = document.createElement("a")
      link.href = url
      link.download = filenameFromDisposition(response.headers.get("content-disposition"))
      document.body.appendChild(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(url)
    } catch (error) {
      console.error("Error downloading XML:", error)
      const message = error instanceof Error ? error.message : "Неизвестная ошибка"
      alert(`Ошибка при создании XML: ${message}`)
    } finally {
      setLoading(false)
    }
  }

  return (
    <Button type="button" variant="outline" onClick={handleDownload} disabled={loading}>
      <Download className={`mr-2 h-4 w-4 ${loading ? "animate-pulse" : ""}`} />
      {loading ? "Создание XML..." : "Скачать УПД XML"}
    </Button>
  )
}
