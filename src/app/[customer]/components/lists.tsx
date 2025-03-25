"use client"
import InvoicesList from "./invoicesList"
import СertificateList from "./certificateList" 
import { useState } from "react"







export default function Lists ({slug}:{slug:string}) {
    const [isOpen, setIsOpen] = useState(false);
    const handleChenge = () => { 
        setIsOpen(!isOpen)
    }

    return (


        <div>
            <div className="flex">
                <button disabled={isOpen} data-selected={isOpen} className="data-[selected=true]:border-b-2 mx-8 pb-2" onClick={handleChenge}>Счета</button>
                <button disabled={!isOpen} data-selected={!isOpen} className="data-[selected=true]:border-b-2 mx-8 pb-2" onClick={handleChenge}>Акты</button>

            </div>
            
           
            
            {isOpen && <InvoicesList slug={slug}/>}
            {!isOpen && <СertificateList slug={slug}/> }
        
        </div>
    )
}