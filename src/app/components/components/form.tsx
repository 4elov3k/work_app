import { IData, Iitem } from '@/app/[customer]/page'
import * as rubles from "rubles"
import './form.css'
import axios from 'axios'
export default async function FormCus ({id}:{id: string}) {


    
    let dataInvoice = await axios.get(`http://127.0.0.1:8090/api/collections/invoices/records/${id}`)
    let dataCustomer = await axios.get(`http://127.0.0.1:8090/api/collections/customers/records/${dataInvoice.data.customer}`)
    let dataServices = await axios.get(`http://127.0.0.1:8090/api/collections/invoices/records/${id}?expand=services`)
    
    
    
    let services = dataServices.data.expand.services
  
    
    let custumer = dataCustomer.data
    
  
    
    let num = dataInvoice.data.number
    let date = dataInvoice.data.date
    let sum = 0
    services.forEach(element => {
        element = Number(element.price)
       sum += element
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
        result += '.' + parts[1];
      
        return result;
      }
      
   return ( 
   
    <div className="container text-black">
        
        <div className="header">
            <p>ИП Мыльникова Любовь Валерьевна</p>
            <p>Адрес: 603146, Нижегородская обл., Нижний Новгород, ул. Головнина, д. 39, кв. 7, тел: 8-905-864445</p>
            <p>ИНН: 526220116209 КПП:</p>
        </div>

        <h1>СЧЕТ №{num} от {date}г.</h1>

        <div>
            <p><strong>Получатель:</strong> ИП Мыльникова Любовь Валерьевна</p>
            <p><strong>Банк получателя:</strong> ООО "Банк Точка"</p>
            <p>Р/с: 40802810164270001108 БИК: 044525104</p>
            <p>К/с: 30101810445745251004</p>
        </div>

        <div>
            <p><strong>Плательщик:</strong> {custumer.fullname}</p>
            <p>ИНН: {custumer.inn}</p>
        </div>

        <table>
            <thead>
                <tr>
                    <th>№</th>
                    <th>Наименование товара</th>
                    <th>Единица измерения</th>
                    <th>Количество</th>
                    <th>Цена</th>
                    <th>Сумма</th>
                </tr>
            </thead>
            <tbody>
                {services.map((service) => (<tr key={service.id}>
                    <td>1</td>
                    <td>{service.name}</td>
                    <td>шт</td>
                    <td>1</td>
                    <td className='whitespace-nowrap w-[1%]'>{convertToCost(service.price)}</td>
                    <td className='whitespace-nowrap w-[1%]'>{convertToCost(service.price)}</td>
                </tr>))}
                
            
            </tbody>
            <tfoot>
                <tr>
                    <td colSpan={5}>Итого:</td>
                    <td>{convertToCost(sum)}</td>
                </tr>
                <tr>
                    <td colSpan={5}>Без налога (НДС):</td>
                    <td>{convertToCost(sum)}</td>
                </tr>
                <tr>
                    <td colSpan={5}>Всего к оплате:</td>
                    <td>{convertToCost(sum)}</td>
                </tr>
            </tfoot>
           
        
        </table>
        <p>Всего наименований {services.length}, на {rubles.rubles(sum)}</p>

        <div className="signature">
            <p className='mb-4'>Руководитель предприятия: Л.В. Мыльникова</p>
            ______________________
            <p>М.П</p>
           
        </div>
        
        
    </div>


)
}