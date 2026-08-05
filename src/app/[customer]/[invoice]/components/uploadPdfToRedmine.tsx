"use client"

import { useState } from "react"
import html2canvas from "html2canvas"
import JsPDF from "jspdf"
import { Loader2, Upload } from "lucide-react"

import { customersAPI, redmineAPI } from "@/lib/api"
import { Button } from "@/components/ui/button"

type Props = {
  customerId: string
  documentType: "invoice" | "act"
  documentId: string
  fileName: string
}

export default function UploadPdfToRedmine({ customerId, documentType, documentId, fileName }: Props) {
  const [loading, setLoading] = useState(false)

  const handleUpload = async () => {
    setLoading(true)
    try {
      const linkResponse = await customersAPI.getRedmineProject(customerId)
      if (!linkResponse.data) {
        alert("Сначала привяжите контрагента к проекту Redmine в карточке контрагента")
        return
      }

      const element = document.querySelector("[data-document-content]") as HTMLElement
      if (!element) {
        alert("Документ на странице не найден")
        return
      }

      const canvas = await html2canvas(element, {
        scale: 1.5,
        useCORS: true,
        logging: false,
        backgroundColor: "#ffffff",
      })

      const imgData = canvas.toDataURL("image/jpeg", 0.92)
      const pdf = new JsPDF({
        orientation: "portrait",
        unit: "mm",
        format: "a4",
      })

      const imgWidth = 210
      const imgHeight = (canvas.height * imgWidth) / canvas.width
      pdf.addImage(imgData, "JPEG", 0, 0, imgWidth, imgHeight)

      const dataUri = pdf.output("datauristring")
      const contentBase64 = dataUri.split(",")[1]
      await redmineAPI.uploadDocumentPdf({
        document_type: documentType,
        document_id: documentId,
        filename: `${fileName}.pdf`,
        content_base64: contentBase64,
      })

      alert("PDF отправлен в файлы проекта Redmine")
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "Не удалось отправить PDF в Redmine"
      if (message.includes("Customer is not linked to a Redmine project")) {
        alert("Сначала привяжите контрагента к проекту Redmine в карточке контрагента")
        return
      }
      console.error("Failed to upload PDF to Redmine:", err)
      alert(message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <Button type="button" variant="outline" onClick={handleUpload} disabled={loading}>
      {loading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Upload className="mr-2 h-4 w-4" />}
      Отправить в файлы Redmine
    </Button>
  )
}
