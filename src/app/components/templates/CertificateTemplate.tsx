import * as rubles from "rubles"
import { Customer, ActWithServices } from "@/lib/api"
import type { Organization } from "@/lib/api.server"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow, TableFooter } from "@/components/ui/table"
import { Card, CardContent } from "@/components/ui/card"
import EditableServiceLine from "./EditableServiceLine"
import { Signature, Stamp } from "./StampAndSignature"

interface CertificateTemplateProps {
  invoice: ActWithServices
  customer: Customer
  docId: string
  organization: Organization
}

function formatSignerName(value?: string): string {
  const parts = (value || "").trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return ""
  if (parts.length === 1) return parts[0]

  const [lastName, ...names] = parts
  const initials = names
    .map((part) => part[0])
    .filter(Boolean)
    .map((letter) => `${letter.toUpperCase()}.`)
    .join(" ")

  return initials ? `${lastName} ${initials}` : lastName
}

export default function CertificateTemplate({ invoice, customer, docId, organization }: CertificateTemplateProps) {
  const services = invoice.services
  const num = invoice.number
  const date = invoice.date
  const customerSignerPosition = customer.contact_position?.trim() || "Директор"
  const customerSignerName = formatSignerName(customer.contact_person)
  const sellerSignerName = formatSignerName(
    [organization.signer.last_name, organization.signer.first_name, organization.signer.middle_name].join(" ")
  )
  const sellerAddress = organization.legal_address || organization.postal_address || ""
  
  // Вычисляем сумму
  let sum = 0
  services.forEach(service => {
    sum += service.amount ?? service.price * (service.qty || 1)
  });
  
  function convertToCost(price: number | string): string {
    let amount: number;
    
    if (typeof price === 'string') {
      amount = parseFloat(price.replace(',', '.'));
    } else {
      amount = price;
    }
  
    const cleanPrice = amount.toFixed(2).replace(/[^\d.]/g, '');
    const parts = cleanPrice.split('.');
    let result = '';
    if (parts.length > 1 && parts[0].length >= 3) {
      result += parts[0].slice(0, -3) + ' ' + parts[0].slice(-3);
    } else {
      result += parts[0];
    }
    result += ',' + parts[1];
  
    return result;
  }
  
  return (
    <Card className="mx-auto bg-white print:shadow-none print:border-none max-w-[210mm] min-h-[297mm] print:min-h-[297mm]">
      <CardContent data-document-content="true" className="p-6 print:p-8 text-black text-xs relative">
        {/* Шапка с реквизитами продавца */}
        <div className="text-left mb-3 space-y-0">
          <p className="text-[10px] font-semibold">{organization.full_name}, ИНН {organization.inn}</p>
          {sellerAddress && <p className="text-[10px]">Адрес: {sellerAddress}</p>}
        </div>

        {/* Заголовок акта */}
        <h1 className="text-base font-bold text-center mb-1">
          Акт № {num} от {date} г.
        </h1>

        {/* Информация о заказчике */}
        <div className="grid grid-cols-2 gap-4 mb-2 text-[11px]">
          <div>
            <p>Заказчик:</p>
            <p>ИНН:</p>
            <p>Адрес:</p>
          </div>
          <div>
            <p>{customer.fullname}</p>
            <p>{customer.inn}</p>
            <p>{customer.address || 'Россия'}</p>
          </div>
        </div>

        {invoice.contract_number && (
          <div className="text-[11px] mb-2">
            <p>Договор: № {invoice.contract_number}</p>
          </div>
        )}

        {/* Таблица работ/услуг */}
        <div className="mb-2">
          <Table>
            <TableHeader>
              <TableRow className="print:border-black h-8">
                <TableHead className="w-[40px] text-[10px] p-1 text-center print:border print:border-black print:bg-white">№</TableHead>
                <TableHead className="text-[10px] p-1 print:border print:border-black print:bg-white">Наименование работ (услуг)</TableHead>
                <TableHead className="text-center w-[70px] text-[10px] p-1 print:border print:border-black print:bg-white">Ед. изм.</TableHead>
                <TableHead className="text-center w-[70px] text-[10px] p-1 print:border print:border-black print:bg-white">Количество</TableHead>
                <TableHead className="text-center w-[90px] text-[10px] p-1 print:border print:border-black print:bg-white">Цена</TableHead>
                <TableHead className="text-center w-[90px] text-[10px] p-1 print:border print:border-black print:bg-white">Сумма</TableHead>
                <TableHead className="not-print w-[76px] p-1"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {services.map((service, index) => (
                <EditableServiceLine
                  key={service.id}
                  docId={docId}
                  docType="act"
                  service={service}
                  index={index}
                />
              ))}
            </TableBody>
            <TableFooter>
              <TableRow className="print:border-black h-7">
                <TableCell colSpan={5} className="text-[10px] p-1 text-right print:border print:border-black">
                  Итого:
                </TableCell>
                <TableCell className="text-[10px] p-1 text-right tabular-nums print:border print:border-black">
                  {convertToCost(sum)}
                </TableCell>
                <TableCell className="not-print p-1"></TableCell>
              </TableRow>
              <TableRow className="print:border-black h-7">
                <TableCell colSpan={5} className="text-[10px] p-1 text-right print:border print:border-black">
                  Без налога (НДС):
                </TableCell>
                <TableCell className="text-[10px] p-1 text-right print:border print:border-black">
                  -
                </TableCell>
                <TableCell className="not-print p-1"></TableCell>
              </TableRow>
              <TableRow className="print:border-black h-7">
                <TableCell colSpan={5} className="text-[10px] p-1 text-right font-semibold print:border print:border-black">
                  Всего (с учетом НДС):
                </TableCell>
                <TableCell className="text-[10px] p-1 text-right font-semibold tabular-nums print:border print:border-black">
                  {convertToCost(sum)}
                </TableCell>
                <TableCell className="not-print p-1"></TableCell>
              </TableRow>
            </TableFooter>
          </Table>
        </div>

        {/* Итого прописью */}
        <p className="text-[11px] mb-4">
          <i>Всего оказано услуг на сумму:</i> {rubles.rubles(sum)}
        </p>
        <p className="text-[11px] mb-4">
          Вышеперечисленные услуги выполнены полностью и в срок. Заказчик претензий по объёму, качеству и срокам оказания услуг не имеет.
        </p>

        {/* Подписи двух сторон */}
        <div className="grid grid-cols-2 gap-8 mt-6">
          {/* Исполнитель */}
          <div>
            <div className="flex items-start justify-between mb-1">
              <div className="text-[11px] flex-shrink-0">Исполнитель:</div>
              <div className="text-[11px] text-right">{sellerSignerName}</div>
            </div>
            <div className="relative flex items-center gap-2 mt-2">
              <div className="border-b border-black flex-1 h-6"></div>
              <Signature />
            </div>
            <p className="text-[9px] text-gray-500 mt-1">(подпись)</p>
          </div>

          {/* Заказчик */}
          <div>
            <div className="flex items-start justify-between mb-1">
              <div className="text-[11px] flex-shrink-0">Заказчик:</div>
              <div className="text-[11px] text-right">{customerSignerName || customerSignerPosition}</div>
            </div>
            <div className="flex items-center gap-2 mt-2">
              <div className="border-b border-black flex-1 h-6"></div>
            </div>
            <p className="text-[9px] text-gray-500 mt-1">(подпись)</p>
          </div>
        </div>

        <div className="relative text-left mt-6">
          <p className="text-[11px]">М.П.</p>
          <Stamp />
        </div>
      </CardContent>
    </Card>
  )
}
