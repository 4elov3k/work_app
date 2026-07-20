import * as rubles from "rubles"
import { Customer, InvoiceWithServices } from "@/lib/api"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow, TableFooter } from "@/components/ui/table"
import { Card, CardContent } from "@/components/ui/card"
import EditableServiceLine from "./EditableServiceLine"

interface InvoiceTemplateProps {
  invoice: InvoiceWithServices
  customer: Customer
  docId: string
}

export default function InvoiceTemplate({ invoice, customer, docId }: InvoiceTemplateProps) {
  const services = invoice.services
  const num = invoice.number
  const date = invoice.date
  
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
        {/* Шапка с реквизитами ИП */}
        <div className="text-center mb-3 space-y-0">
          <p className="text-[11px] font-semibold">ИП Мыленкова Любовь Валерьевна</p>
          <p className="text-[10px]">
            Адрес: 603136, г. Нижний Новгород ул, Маршала Рокоссовского, д. 2к1, кв 135
          </p>
          <p className="text-[10px]">тел: 8-905-864445</p>
          <p className="text-[10px]">ИНН: 526220116209</p>
        </div>

        <div className="border-t border-black my-3"></div>

        {/* Заголовок счета */}
        <h1 className="text-base font-bold text-center mb-3">
          СЧЕТ № {num} от {date} г.
        </h1>

        {/* Реквизиты получателя */}
        <div className="mb-3 space-y-0 text-[11px]">
          <p><strong>Получатель:</strong> ИП Мыленкова Любовь Валерьевна</p>
          <p><strong>Банк получателя:</strong> ООО &quot;Банк Точка&quot;</p>
          <p>Р/с: 40802810164270001108 БИК: 044525104</p>
          <p>К/с: 30101810445745251004</p>
        </div>

        <div className="border-t border-black my-3"></div>

        {/* Реквизиты плательщика */}
        <div className="mb-3 space-y-0 text-[11px]">
          <p><strong>Плательщик:</strong> {customer.fullname}</p>
          <p>ИНН: {customer.inn}</p>
          {customer.address && <p>Адрес: {customer.address}</p>}
        </div>

        {/* Таблица услуг */}
        <div className="mb-2">
          <Table>
            <TableHeader>
              <TableRow className="print:border-black h-8">
                <TableHead className="w-[40px] text-[10px] p-1 text-center print:border print:border-black print:bg-white">№</TableHead>
                <TableHead className="text-[10px] p-1 print:border print:border-black print:bg-white">Наименование товара</TableHead>
                <TableHead className="text-center w-[70px] text-[10px] p-1 print:border print:border-black print:bg-white">Ед. изм.</TableHead>
                <TableHead className="text-center w-[70px] text-[10px] p-1 print:border print:border-black print:bg-white">Кол-во</TableHead>
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
                  docType="invoice"
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
                  Всего к оплате:
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
          Всего наименований {services.length}, на сумму {rubles.rubles(sum)}
        </p>

        <div className="border-t border-black my-4"></div>

        {/* Подпись */}
        <div className="mt-6">
          <div className="flex justify-between items-start mb-2">
            <div className="text-[11px]">
              <p className="mb-3">Руководитель предприятия:</p>
              <div className="border-b border-black w-48 h-6"></div>
              <p className="text-[9px] text-gray-500 mt-1">(подпись)</p>
            </div>
            <div className="text-[11px] mt-8">
              Л.В. Мыленкова
            </div>
          </div>
          <p className="text-[11px] mt-6">М.П.</p>
        </div>
      </CardContent>
    </Card>
  )
}
