"use client"
import axios from "axios";
import { IData, Iitem } from "../page";

import { useEffect, useState } from "react";
import Link from "next/link";

export default function InvoicesList ({slug}:{slug:string}){
   
    const [fetchItems, setFetchItems] = useState<Iitem[]>([{id:"1"}])
    const [list, setList] = useState<boolean>(false)
    const handleClickAdd = (evet: React.MouseEvent<HTMLDivElement>) => {
        addInvoice()
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

      const defaultItem: Iitem = {
        "customer": `${slug}`,
        "number": "123",
        "date": "24 июля 2014"
    };
    function addInvoice() {
        
		fetch("http://127.0.0.1:8090/api/collections/invoices/records", {
            method:"POST",
            headers: {
                'Content-Type': 'application/json;charset=utf-8'
              },
            body: JSON.stringify(defaultItem)
        }).then(() => setList(!list));
        
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
                onClick={handleClickAdd}>
                Добавить счет 
            </div>
           
               
      </> 
    );
}