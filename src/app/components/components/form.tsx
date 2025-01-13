import './form.css'

export default async function FormCus () {
    let num = "0000"
    let date = "20 августа 2025"
    let name = "Название организации"
    let inn = "123456789"
    let service = {
        name: "Оплата по договору 481 от 21 августа 2024 остаточной суммы",
        unit: "шт",
        count: 1,
        price: 39786,
    };
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
            <p><strong>Плательщик:</strong> {name}</p>
            <p>ИНН: {inn}</p>
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
                <tr>
                    <td>1</td>
                    <td>{service.name}</td>
                    <td>шт</td>
                    <td>1</td>
                    <td>{service.price}</td>
                    <td>{convertToCost(service.price)}</td>
                </tr>
                <tr>
                    <td>1</td>
                    <td>{service.name}</td>
                    <td>шт</td>
                    <td>1</td>
                    <td>{service.price}</td>
                    <td>{convertToCost(service.price)}</td>
                </tr>
            </tbody>
            <tfoot>
                <tr>
                    <td colSpan="5">Итого:</td>
                    <td>31,250.00</td>
                </tr>
                <tr>
                    <td colSpan="5">Без налога (НДС):</td>
                    <td>31,250.00</td>
                </tr>
                <tr>
                    <td colSpan="5">Всего к оплате:</td>
                    <td>31,250.00</td>
                </tr>
            </tfoot>
        </table>

        <p>Всего наименований 1, на сумму тридцать одна тысяча двести пятьдесят рублей 00 копеек.</p>

        <div className="signature">
            <p>Руководитель предприятия: Л.В. Мыльникова</p>
           
        </div>
    </div>


)
}