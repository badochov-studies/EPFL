`timescale 1ps/1ps

module counterTestBench;

  reg clock,reset, s_enable, s_direction;
  
  initial
    begin
      reset = 1'b1;
      clock = 1'b0;
      repeat (4) #5 clock = ~clock;
      reset = 1'b0;
      forever #5 clock = ~clock;
    end

  wire [7:0] s_value;
  counter #(.WIDTH(8)) dut 
    ( .reset(reset),
      .clock(clock),
      .enable(s_enable),
      .direction(s_direction),
      .counterValue(s_value));

  always @(negedge clock)
    begin
      s_enable    <= (reset == 1'b1) ? 1'b0 : ~s_enable;
      s_direction <= (reset == 1'b1) ? 1'b1 :
                     (s_enable == 1'b0 && s_value == 8'd55) ? 1'b0 : s_direction;
    end
  
  initial
    begin
      s_direction = 1'b1;
      @(negedge reset);
      forever @(negedge clock) if (s_direction == 1'b0 && s_value == 8'd127) $finish;
      $finish;
    end
      
  
  initial
    begin
      $dumpfile("counterSignals.vcd");
      $dumpvars(1,dut);
    end
endmodule
