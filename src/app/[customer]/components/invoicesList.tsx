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
	const handleClickAdd = (event: React.MouseEvent<HTMLButtonElement>) => {
		
		const dataToFetch: Iitem = {
			number: number,
			date: date,
			customer: `${slug}`
		}
		
        addInvoice(dataToFetch)
		setNumber("")
		setDate("")
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
            
          
           <div className="add-btn text-center cursor-pointer w-[400px]"
                onClick={handleClickOpen}>
                Добавить счет 
            </div>
            {isOpen && (
				
			<>
				<input type="text" onChange={(event: React.ChangeEvent<HTMLInputElement>) => {setNumber(event.target.value)} } value={number} placeholder="Номер счета" className="border mt-24 ml-6 border-gray-50 bg-[var(--foreground)]"/>
				<input type="text" onChange={(event: React.ChangeEvent<HTMLInputElement>) => {setDate(event.target.value)} } value={date} placeholder="Дата счета" className="border mt-24 ml-6 border-gray-50 bg-[var(--foreground)]"/>
				<button className="text-center" onClick={handleClickAdd}>Добавить</button>	
			</>
            )
            
            }
           
               
      </> 
    );
}