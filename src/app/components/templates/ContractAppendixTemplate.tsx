import * as rubles from "rubles"
import { Customer, Contract, ContractAppendixWithLines } from "@/lib/api"
import type { Organization } from "@/lib/api.server"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow, TableFooter } from "@/components/ui/table"
import { Card, CardContent } from "@/components/ui/card"
import { Signature, Stamp } from "./StampAndSignature"

interface ContractAppendixTemplateProps {
  appendix: ContractAppendixWithLines
  contract: Contract
  customer: Customer
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

function formatMoney(value: number): string {
  return value.toLocaleString("ru-RU", { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

export default function ContractAppendixTemplate({
  appendix,
  contract,
  customer,
  organization,
}: ContractAppendixTemplateProps) {
  const customerSignerPosition = customer.contact_position?.trim() || "Директор"
  const customerSignerName = formatSignerName(customer.contact_person)
  const sellerSignerName = formatSignerName(
    [organization.signer.last_name, organization.signer.first_name, organization.signer.middle_name].join(" ")
  )

  const sections: { section: string; lines: typeof appendix.lines; subtotal: number }[] = []
  for (const line of appendix.lines) {
    const key = line.section || "Прочее"
    let group = sections.find((s) => s.section === key)
    if (!group) {
      group = { section: key, lines: [], subtotal: 0 }
      sections.push(group)
    }
    group.lines.push(line)
    group.subtotal += line.amount
  }

  return (
    <Card className="mx-auto bg-white print:shadow-none print:border-none max-w-[210mm] min-h-[297mm] print:min-h-[297mm]">
      <CardContent data-document-content="true" className="p-6 print:p-8 text-black text-xs relative">
        <div className="text-left mb-3 space-y-0">
          <p className="text-[10px] font-semibold">{organization.full_name}, ИНН {organization.inn}</p>
        </div>

        <h1 className="text-base font-bold text-center mb-1">
          Приложение № {appendix.number} к Договору № {contract.number} от {contract.start_date || appendix.date} г.
        </h1>
        <p className="text-[11px] text-center mb-4">Сметный расчёт стоимости работ от {appendix.date} г.</p>

        <div className="grid grid-cols-2 gap-4 mb-4 text-[11px]">
          <div>
            <p>Исполнитель:</p>
            <p>Заказчик:</p>
          </div>
          <div>
            <p>{organization.full_name}</p>
            <p>{customer.fullname}</p>
          </div>
        </div>

        {sections.map((group) => (
          <div key={group.section} className="mb-3">
            <p className="font-semibold text-[11px] mb-1">{group.section}</p>
            <Table>
              <TableHeader>
                <TableRow className="print:border-black h-8">
                  <TableHead className="w-[36px] text-[10px] p-1 text-center print:border print:border-black print:bg-white">№</TableHead>
                  <TableHead className="text-[10px] p-1 print:border print:border-black print:bg-white">Наименование работ (услуг)</TableHead>
                  <TableHead className="text-center w-[60px] text-[10px] p-1 print:border print:border-black print:bg-white">Ед. изм.</TableHead>
                  <TableHead className="text-center w-[60px] text-[10px] p-1 print:border print:border-black print:bg-white">Кол-во</TableHead>
                  <TableHead className="text-center w-[80px] text-[10px] p-1 print:border print:border-black print:bg-white">Цена</TableHead>
                  <TableHead className="text-center w-[90px] text-[10px] p-1 print:border print:border-black print:bg-white">Сумма</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {group.lines.map((line, index) => (
                  <TableRow key={line.id} className="print:border-black">
                    <TableCell className="text-[10px] p-1 text-center print:border print:border-black">{index + 1}</TableCell>
                    <TableCell className="text-[10px] p-1 print:border print:border-black">{line.title}</TableCell>
                    <TableCell className="text-[10px] p-1 text-center print:border print:border-black">{line.unit}</TableCell>
                    <TableCell className="text-[10px] p-1 text-center tabular-nums print:border print:border-black">{line.qty}</TableCell>
                    <TableCell className="text-[10px] p-1 text-right tabular-nums print:border print:border-black">{formatMoney(line.price)}</TableCell>
                    <TableCell className="text-[10px] p-1 text-right tabular-nums print:border print:border-black">{formatMoney(line.amount)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
              <TableFooter>
                <TableRow className="print:border-black h-7">
                  <TableCell colSpan={5} className="text-[10px] p-1 text-right font-medium print:border print:border-black">
                    Итого по разделу:
                  </TableCell>
                  <TableCell className="text-[10px] p-1 text-right font-medium tabular-nums print:border print:border-black">
                    {formatMoney(group.subtotal)}
                  </TableCell>
                </TableRow>
              </TableFooter>
            </Table>
          </div>
        ))}

        <div className="text-right text-[12px] font-semibold my-4">
          Всего по смете: {formatMoney(appendix.total_amount)} руб.
        </div>
        <p className="text-[11px] mb-6">
          <i>Итого:</i> {rubles.rubles(appendix.total_amount)}, НДС не облагается.
        </p>

        <div className="grid grid-cols-2 gap-8 mt-6">
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
