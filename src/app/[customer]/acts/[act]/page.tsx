import FormCus from "@/app/components/components/form"
import Link from "next/link"
import { notFound } from "next/navigation"
import Print from "../../[invoice]/components/print"
import DownloadPdf from "../../[invoice]/components/downloadPdf"
import UploadPdfToRedmine from "../../[invoice]/components/uploadPdfToRedmine"
import DownloadXml from "./components/downloadXml"
import { ArrowLeft, Home } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { actsAPI, ApiError } from "@/lib/api.server"
import AddLine from "../../components/addLine"
import EditDocument from "../../components/editDocument"
import DocumentActionsMenu from "../../components/documentActionsMenu"

export default async function Page({
  params
}: {
  params: Promise<{ act: string; customer: string }>
}) {
  const { act: actId, customer: customerId } = await params

  let actData
  try {
    actData = await actsAPI.getById(actId)
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      notFound()
    }
    throw err
  }
  const act = actData.data
  const fileName = `Акт_${act.number}_${act.date.replace(/\./g, "-")}`

  return (
    <div className="container mx-auto py-8 px-4 max-w-5xl">
      <div className="not-print mb-6">
        <div className="flex flex-wrap gap-4 items-center mb-6">
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
          <DocumentActionsMenu>
            <EditDocument
              docType="act"
              docId={actId}
              number={act.number}
              date={act.date}
              status={act.status}
              archived={act.archived}
            />
            {!act.archived && <AddLine docId={actId} docType="act" />}
            <DownloadXml actId={actId} />
            <DownloadPdf fileName={fileName} />
            <UploadPdfToRedmine customerId={customerId} documentType="act" documentId={actId} fileName={fileName} />
            <Print />
          </DocumentActionsMenu>
        </div>
        <Separator />
      </div>

      <FormCus id={actId} type="act" />
    </div>
  )
}
