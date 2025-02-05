"use client"
import axios from "axios";
import { IData, Iitem } from "../page";

import React, { useEffect, useState } from "react";
import Link from "next/link";

export default function InvoicesList ({slug}:{slug:string}){
   
    const [fetchItems, setFetchItems] = useState<Iitem[]>([{id:"1"}])
    const [list, setList] = useState<boolean>(false)
    const [isOpen, setIsOpen] = useState<boolean>(false)
    const [number, setNumber] =useState<string>("")
    const [date, setDate] =useState<string>("")
	const [serviceName, setServiceName] =useState<string>("")
	const [servicePrice, setServicePrice] =useState<string>("")
	const [serviceDefault, setServiceDefault] =useState<boolean>(true)
    const handleClickOpen = (event: React.MouseEvent<HTMLDivElement>) => {
      setIsOpen(!isOpen)
    }
    

  
    useEffect(()=>{
        axios.get(`http://127.0.0.1:8090/api/collections/invoices/records?filter=(customer='${slug}')`).then(resp => resp.data).then(data => {
          if(!data.items) {
            setFetchItems([{id:"1"}])
            return
          }
          setFetchItems(data.items)
          setList(!list)
        })
      }, [])

      useEffect(() => {
        axios.get(`http://127.0.0.1:8090/api/collections/invoices/records?filter=(customer='${slug}')`).then(resp => resp.data).then(data => {
            if(!data.items) {
              setFetchItems([{id:"1"}])
              return
            }
            setFetchItems(data.items)
          })
      }, [list])

    
    function addInvoice(dataToFetch:Iitem) {
        
		fetch("http://127.0.0.1:8090/api/collections/invoices/records", {
            method:"POST",
            headers: {
                'Content-Type': 'application/json;charset=utf-8'
              },
            body: JSON.stringify(dataToFetch)
        }).then(() => setList(!list));
        
	}
	function addService(serviceToFetch:Iitem) {
        
		fetch("http://127.0.0.1:8090/api/collections/services/records", {
            method:"POST",
            headers: {
                'Content-Type': 'application/json;charset=utf-8'
              },
            body: JSON.stringify(serviceToFetch)
        }).then(() => setList(!list));
        
	}
	const newDate = date.split("-").reverse().join(".")
	const handleClickAdd = (event: React.MouseEvent<HTMLButtonElement>) => {
		
		const dataToFetch: Iitem = {
			number: number,
			date: newDate,
			customer: `${slug}`
		}
		const serviceToFetch:Iitem = {
			name: serviceName,
			price: servicePrice,
			
		}
		addService(serviceToFetch)
        addInvoice(dataToFetch)
		setNumber("")
		setDate("")
		setServiceName("")
		setServicePrice("")
		setIsOpen(!isOpen)
		
      }
	  

    return (
       <>
           {fetchItems.map((item) => (
               <div key={item.id} className="custumer">
               <div>
                   <strong><Link href={`/${slug}`+'/'+ `${item.id ?? '/'}`}>{item.number}</Link></strong>
               </div>
               <div>
                   {item.date}
               </div>
             </div>
             
           ))}
            
          
			{!isOpen &&(
					<div className="add-btn text-center cursor-pointer w-[400px]"
					onClick={handleClickOpen}>
						Добавить счет 
					</div>
				)
			}

			{isOpen && (

					<div className="flex border border-gray-50 p-8 pt-0 mt-8 gap-4 flex-col w-1/2 mx-auto">
						<div className="add-btn text-center cursor-pointer w-[400px]"
						onClick={handleClickOpen}>
							Отменить 
						</div>
						<input type="text" onChange={(event: React.ChangeEvent<HTMLInputElement>) => {setNumber(event.target.value)} } value={number} placeholder="Номер счета" className="border  border-gray-50 bg-[var(--foreground)]"/>
						<input type="date" pattern="dd-mm-yyyy" onChange={(event: React.ChangeEvent<HTMLInputElement>) => {setDate(event.target.value)} } value={date} placeholder="Дата счета" className="border  border-gray-50 bg-[var(--foreground)]"/>
						<input type="text" onChange={(event: React.ChangeEvent<HTMLInputElement>) => {setServiceName(event.target.value)} } value={serviceName} placeholder="Название услуги" className="border  border-gray-50 bg-[var(--foreground)]"/>
						<input type="text" onChange={(event: React.ChangeEvent<HTMLInputElement>) => {setServicePrice(event.target.value)} } value={servicePrice} placeholder="Цена" className="border  border-gray-50 bg-[var(--foreground)]"/>
						<input type="checkbox" onChange={(event: React.ChangeEvent<HTMLInputElement>) => {setServiceDefault(event.target.checked)}}/>
						<button className="text-center btn" onClick={handleClickAdd}>Добавить</button>	

					</div>
				)

			}
           
               
      </> 
    );
}