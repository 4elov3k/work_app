"use client"
import { Button } from "@/components/ui/button"
import { Download } from "lucide-react"
import html2canvas from "html2canvas"
import JsPDF from "jspdf"
import { useState } from "react"

interface DownloadPdfProps {
  fileName: string
}

export default function DownloadPdf({ fileName }: DownloadPdfProps) {
  const [loading, setLoading] = useState(false)

  const handleDownload = async () => {
    setLoading(true)
    try {
      // Находим элемент с документом
      const element = document.querySelector('[data-document-content]') as HTMLElement
      if (!element) {
        console.error('Document content not found')
        return
      }

      // Создаем canvas из HTML
      const canvas = await html2canvas(element, {
        scale: 2,
        useCORS: true,
        logging: false,
        backgroundColor: '#ffffff'
      })

      // Создаем PDF
      const imgData = canvas.toDataURL('image/png')
      const pdf = new JsPDF({
        orientation: 'portrait',
        unit: 'mm',
        format: 'a4'
      })

      const imgWidth = 210 // A4 width in mm
      const imgHeight = (canvas.height * imgWidth) / canvas.width

      pdf.addImage(imgData, 'PNG', 0, 0, imgWidth, imgHeight)
      pdf.save(`${fileName}.pdf`)
    } catch (error) {
      console.error('Error generating PDF:', error)
      alert('Ошибка при создании PDF')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Button onClick={handleDownload} disabled={loading}>
      {loading ? (
        <>
          <Download className="mr-2 h-4 w-4 animate-pulse" />
          Создание PDF...
        </>
      ) : (
        <>
          <Download className="mr-2 h-4 w-4" />
          Скачать PDF
        </>
      )}
    </Button>
  )
}
