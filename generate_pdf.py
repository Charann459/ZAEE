import sys
import json
from fpdf import FPDF
from confluent_kafka import Consumer, KafkaError

def main():
    conf = {
        'bootstrap.servers': 'kafka:29092',
        'group.id': 'pdf_generator_group',
        'auto.offset.reset': 'earliest'
    }
    
    consumer = Consumer(conf)
    consumer.subscribe(['zaee_output'])
    
    pdf = FPDF()
    pdf.add_page()
    pdf.set_font("Arial", size=10)
    pdf.cell(200, 10, txt="ZAEE Engine Output - Phase 3 Verification Report", ln=1, align='C')
    pdf.ln(10)
    
    print("Listening for messages on zaee_output...")
    
    messages = []
    
    try:
        # Collect exactly 55 messages to ensure we see the cold start exit
        while len(messages) < 55:
            msg = consumer.poll(1.0)
            if msg is None:
                continue
            if msg.error():
                if msg.error().code() == KafkaError._PARTITION_EOF:
                    continue
                else:
                    print(msg.error())
                    break
            
            val = msg.value().decode('utf-8')
            messages.append(val)
            print(f"Collected {len(messages)}/55 messages")
            
    except KeyboardInterrupt:
        pass
    finally:
        consumer.close()

    for idx, m in enumerate(messages):
        # Format the JSON nicely
        try:
            parsed = json.loads(m)
            formatted = json.dumps(parsed, indent=2)
        except:
            formatted = m
            
        # Write to PDF
        pdf.set_font("Arial", 'B', 9)
        pdf.cell(200, 8, txt=f"Message {idx+1}:", ln=1)
        pdf.set_font("Courier", size=8)
        
        # Replace fancy quotes or unicode that FPDF doesn't like by default
        formatted = formatted.encode('latin-1', 'replace').decode('latin-1')
        
        for line in formatted.split('\n'):
            pdf.cell(200, 5, txt=line, ln=1)
        pdf.ln(5)
        
    pdf.output("/app/verification_report.pdf")
    print("Saved report to verification_report.pdf")

if __name__ == "__main__":
    main()
