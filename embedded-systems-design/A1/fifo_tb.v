`timescale 1ps / 1ps
`include "fifo.v"

module fifoTestBench;

  reg clock, reset, push = 0, pop = 0;
  reg [32 - 1 : 0] pushData = 0;
  wire full, empty;
  wire [32 - 1 : 0] popData;
  // 0 - push, 1 - pull
  reg phase = 0;

  initial begin
    reset = 1'b1;
    clock = 1'b0;
    repeat (4) #5 clock = ~clock;
    reset = 1'b0;
    forever #5 clock = ~clock;
  end

  fifo #(
      .nrOfEntries(16),
      .bitWidth(32)
  ) dut (
      .reset(reset),
      .clock(clock),
      .push(push),
      .pop(pop),
      .pushData(pushData),
      .full(full),
      .empty(empty),
      .popData(popData)
  );

  always @(negedge clock) begin
    if (phase == 0) begin
      push = 1;
      pop = 0;
      pushData = pushData + 1;
    end else begin
      push = 0;
      pop = 1;
    end
  end

  initial begin
    @(negedge reset);
    forever
    @(negedge clock) begin
      if (phase == 0 && full == 1'b1) phase = 1;
      else if (phase == 1 && empty == 1'b1) $finish;
    end
  end

  initial begin
    $dumpfile("popSignals.vcd");
    $dumpvars(1, dut);
  end
endmodule
