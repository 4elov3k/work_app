import FormCus from "@/app/components/components/form"
import Link from "next/link"
import Print from "../../[invoice]/components/print"
import DownloadPdf from "../../[invoice]/components/downloadPdf"
import { ArrowLeft, Download, Home } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { actsAPI } from "@/lib/api.server"
import AddLine from "../../components/addLine"
import EditDocument from "../../components/editDocument"

export default async function Page({
  params
}: {
  params: Promise<{ act: string; customer: string }>
}) {
  const { act: actId, customer: customerId } = await params

  const actData = await actsAPI.getById(actId)
  const act = actData.data
  const fileName = `Акт_${act.number}_${act.date.replace(/\./g, "-")}`
  const apiBase = process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8080/api"
  const xmlUrl = `${apiBase}/acts/${actId}/export/upd-xml`

  return (
    <div className="container mx-auto py-8 px-4 max-w-5xl">
      <div className="not-print mb-6">
        <div className="flex gap-4 items-center mb-6">
          <Link href={`/${customerId}`}>
            <Button variant="outline">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Назад
            </Button>
          </Link>
          <Link href="/">
            <Button variant="outline">
              <Home className="mr-2 h-4 w-4" />
              Главная
            </Button>
          </Link>
          {act.archived && <Badge variant="secondary">В архиве</Badge>}
          <div className="flex-1"></div>
          <EditDocument
            docType="act"
            docId={actId}
            number={act.number}
            date={act.date}
            status={act.status}
            archived={act.archived}
          />
          {!act.archived && <AddLine docId={actId} docType="act" />}
          <Button asChild variant="outline">
            <a href={xmlUrl}>
              <Download className="mr-2 h-4 w-4" />
              XML СБИС
            </a>
          </Button>
          <DownloadPdf fileName={fileName} />
          <Print />
        </div>
        <Separator />
      </div>

      <FormCus id={actId} type="act" />
    </div>
  )
}
