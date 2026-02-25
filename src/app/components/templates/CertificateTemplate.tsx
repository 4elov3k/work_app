import * as rubles from "rubles"
import { Customer, ActWithServices } from "@/lib/api"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow, TableFooter } from "@/components/ui/table"
import { Card, CardContent } from "@/components/ui/card"

interface CertificateTemplateProps {
  invoice: ActWithServices
  customer: Customer
}

export default function CertificateTemplate({ invoice, customer }: CertificateTemplateProps) {
  const services = invoice.services
  const num = invoice.number
  const date = invoice.date
  
  // Вычисляем сумму
  let sum = 0
  services.forEach(service => {
    sum += service.price
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
        <div className="text-left mb-3 space-y-0">
          <p className="text-[10px] font-semibold">ИП Мыльникова Любовь Валерьевна, ИНН 526220116209</p>
          <p className="text-[10px]">
            Адрес: 603146, Нижегородская обл, г. Нижний Новгород, ул. Головнина, д.39, кв. 7
          </p>
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

        <div className="text-[11px] mb-2">
          <p>Договор: {invoice.contract_number || 'Основной'}</p>
        </div>

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
              </TableRow>
            </TableHeader>
            <TableBody>
              {services.map((service, index) => (
                <TableRow key={service.id} className="print:border-black h-7">
                  <TableCell className="text-[10px] p-1 text-center print:border print:border-black">{index + 1}</TableCell>
                  <TableCell className="text-[10px] p-1 print:border print:border-black">{service.name}</TableCell>
                  <TableCell className="text-[10px] p-1 text-center print:border print:border-black">шт</TableCell>
                  <TableCell className="text-[10px] p-1 text-center print:border print:border-black">1</TableCell>
                  <TableCell className="text-[10px] p-1 text-right tabular-nums print:border print:border-black">
                    {convertToCost(service.price)}
                  </TableCell>
                  <TableCell className="text-[10px] p-1 text-right tabular-nums print:border print:border-black">
                    {convertToCost(service.price)}
                  </TableCell>
                </TableRow>
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
              </TableRow>
              <TableRow className="print:border-black h-7">
                <TableCell colSpan={5} className="text-[10px] p-1 text-right print:border print:border-black">
                  Без налога (НДС):
                </TableCell>
                <TableCell className="text-[10px] p-1 text-right print:border print:border-black">
                  -
                </TableCell>
              </TableRow>
              <TableRow className="print:border-black h-7">
                <TableCell colSpan={5} className="text-[10px] p-1 text-right font-semibold print:border print:border-black">
                  Всего (с учетом НДС):
                </TableCell>
                <TableCell className="text-[10px] p-1 text-right font-semibold tabular-nums print:border print:border-black">
                  {convertToCost(sum)}
                </TableCell>
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
              <div className="text-[11px] text-right">Л. В. Мыльникова</div>
            </div>
            <div className="flex items-center gap-2 mt-2">
              <div className="border-b border-black flex-1 h-6"></div>
            </div>
            <p className="text-[9px] text-gray-500 mt-1">(подпись)</p>
          </div>

          {/* Заказчик */}
          <div>
            <div className="flex items-start justify-between mb-1">
              <div className="text-[11px] flex-shrink-0">Заказчик:</div>
              <div className="text-[11px] text-right">{customer.name}</div>
            </div>
            <div className="flex items-center gap-2 mt-2">
              <div className="border-b border-black flex-1 h-6"></div>
            </div>
            <p className="text-[9px] text-gray-500 mt-1">(подпись)</p>
          </div>
        </div>

        <div className="text-left mt-6">
          <p className="text-[11px]">М.П.</p>
        </div>
      </CardContent>
    </Card>
  )
}
